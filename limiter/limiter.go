package limiter

import (
	"errors"
	"strings"
	"sync"
	"time"

	panel "github.com/wyx2685/v2node/api/v2board"
	"github.com/wyx2685/v2node/common/format"
	"github.com/wyx2685/v2node/common/rate"
)

var limiterMap sync.Map

func Init() {
}

type Limiter struct {
	Nodetype      string         // Node type, e.g. "v2ray", "trojan", "shadowsocks"
	SpeedLimit    int            // Node speed limit in Mbps
	UUIDtoUID     sync.Map       // Key: UUID, value: Uid
	UserLimit     sync.Map       // Key: TagUUID value: *UserLimitInfo
	SpeedLimiter  sync.Map       // key: TagUUID, value: *rate.DynamicBucket
	deviceTracker *DeviceTracker // Tracks online devices per user
}

type UserLimitInfo struct {
	UID               int
	SpeedLimit        int
	DeviceLimit       int
	DynamicSpeedLimit int
	ExpireTime        int64
	OverLimit         bool
}

type DeviceTracker struct {
	onlineIPs  sync.Map // Key: taguuid:ip -> uid
	oldOnline  sync.Map // Key: ip -> uid
	aliveCount sync.Map // Key: uid -> count
}

func NewDeviceTracker(aliveList map[int]int) *DeviceTracker {
	dt := &DeviceTracker{}
	for uid, ip := range aliveList {
		dt.aliveCount.Store(uid, ip)
	}
	return dt
}

func (dt *DeviceTracker) TrackDevice(taguuid, ip string, uid, deviceLimit int) bool {
	key := taguuid + ":" + ip

	if existingUID, loaded := dt.onlineIPs.LoadOrStore(key, uid); loaded {
		if existingUID.(int) == uid {
			return false
		}
		return true
	}

	if deviceLimit > 0 {
		countVal, _ := dt.aliveCount.Load(uid)
		count := 0
		if countVal != nil {
			count = countVal.(int)
		}
		if count >= deviceLimit {
			dt.onlineIPs.Delete(key)
			return true
		}
	}

	if oldUID, ok := dt.oldOnline.Load(ip); ok && oldUID.(int) == uid {
		dt.oldOnline.Delete(ip)
	}

	return false
}

func (dt *DeviceTracker) UpdateAliveList(newAlive map[int]int) {
	existing := make(map[int]struct{})
	dt.aliveCount.Range(func(key, _ interface{}) bool {
		existing[key.(int)] = struct{}{}
		return true
	})

	for uid := range existing {
		if _, exists := newAlive[uid]; !exists {
			dt.aliveCount.Delete(uid)
		}
	}

	for uid, count := range newAlive {
		dt.aliveCount.Store(uid, count)
	}
}

func (dt *DeviceTracker) GetOnlineDevices() []panel.OnlineUser {
	result := make([]panel.OnlineUser, 0, 64)

	dt.oldOnline.Range(func(key, value interface{}) bool {
		dt.oldOnline.Delete(key)
		return true
	})

	toDelete := make([]string, 0, 64)
	dt.onlineIPs.Range(func(key, value interface{}) bool {
		toDelete = append(toDelete, key.(string))
		return true
	})

	for _, ipKey := range toDelete {
		if uidVal, ok := dt.onlineIPs.LoadAndDelete(ipKey); ok {
			uid := uidVal.(int)
			ip := ipKey[strings.LastIndex(ipKey, ":")+1:]
			dt.oldOnline.Store(ip, uid)
			result = append(result, panel.OnlineUser{UID: uid, IP: ip})
		}
	}

	return result
}

func (dt *DeviceTracker) DeleteUser(taguuid string) {
	dt.onlineIPs.Range(func(key, value interface{}) bool {
		if strings.HasPrefix(key.(string), taguuid+":") {
			dt.onlineIPs.Delete(key)
		}
		return true
	})
}

func AddLimiter(nodetype string, tag string, users []panel.UserInfo, aliveList map[int]int) *Limiter {
	l := &Limiter{
		Nodetype:      nodetype,
		deviceTracker: NewDeviceTracker(aliveList),
	}
	for i := range users {
		l.UUIDtoUID.Store(users[i].Uuid, users[i].Id)
		userLimit := &UserLimitInfo{
			UID:       users[i].Id,
			OverLimit: false,
		}
		if users[i].SpeedLimit != 0 {
			userLimit.SpeedLimit = users[i].SpeedLimit
		}
		if users[i].DeviceLimit != 0 {
			userLimit.DeviceLimit = users[i].DeviceLimit
		}
		l.UserLimit.Store(format.UserTag(tag, users[i].Uuid), userLimit)
	}
	limiterMap.Store(tag, l)
	return l
}

func GetLimiter(tag string) (*Limiter, error) {
	if info, ok := limiterMap.Load(tag); ok {
		return info.(*Limiter), nil
	}
	return nil, errors.New("not found")
}

func DeleteLimiter(tag string) {
	limiterMap.Delete(tag)
}

func (l *Limiter) UpdateUser(tag string, added []panel.UserInfo, deleted []panel.UserInfo, modified []panel.UserInfo) {
	for i := range deleted {
		taguuid := format.UserTag(tag, deleted[i].Uuid)
		l.UserLimit.Delete(taguuid)
		l.SpeedLimiter.Delete(taguuid)
		l.UUIDtoUID.Delete(deleted[i].Uuid)
		l.deviceTracker.DeleteUser(taguuid)
	}
	for i := range modified {
		taguuid := format.UserTag(tag, modified[i].Uuid)
		if v, ok := l.UserLimit.Load(taguuid); ok {
			u := v.(*UserLimitInfo)
			u.SpeedLimit = modified[i].SpeedLimit
			u.DeviceLimit = modified[i].DeviceLimit
		}
		limit := int64(determineSpeedLimit(l.SpeedLimit, modified[i].SpeedLimit)) * 1000000 / 8
		if limit > 0 {
			if v, ok := l.SpeedLimiter.Load(taguuid); ok {
				d := v.(*rate.DynamicBucket)
				d.Update(limit)
			} else {
				d := rate.NewDynamicBucket(limit)
				l.SpeedLimiter.Store(taguuid, d)
			}
		} else {
			l.SpeedLimiter.Delete(taguuid)
		}
	}
	for i := range added {
		userLimit := &UserLimitInfo{
			UID:       added[i].Id,
			OverLimit: false,
		}
		if added[i].SpeedLimit != 0 {
			userLimit.SpeedLimit = added[i].SpeedLimit
		}
		if added[i].DeviceLimit != 0 {
			userLimit.DeviceLimit = added[i].DeviceLimit
		}
		l.UserLimit.Store(format.UserTag(tag, added[i].Uuid), userLimit)
		l.UUIDtoUID.Store(added[i].Uuid, added[i].Id)
	}
}

func (l *Limiter) UpdateDynamicSpeedLimit(tag, uuid string, limit int, expire time.Time) error {
	taguuid := format.UserTag(tag, uuid)
	if v, ok := l.UserLimit.Load(taguuid); ok {
		info := v.(*UserLimitInfo)
		info.DynamicSpeedLimit = limit
		info.ExpireTime = expire.Unix()
		return nil
	}
	return errors.New("not found")
}

func (l *Limiter) CheckLimit(taguuid string, ip string, noUDPsource bool) (*rate.DynamicBucket, bool) {
	ip = strings.TrimPrefix(ip, "::ffff:")

	nodeLimit := l.SpeedLimit
	userLimit := 0
	deviceLimit := 0
	var uid int

	v, ok := l.UserLimit.Load(taguuid)
	if !ok {
		return nil, true
	}
	u := v.(*UserLimitInfo)
	deviceLimit = u.DeviceLimit
	uid = u.UID

	now := time.Now().Unix()
	if u.ExpireTime != 0 && u.ExpireTime < now {
		if u.SpeedLimit != 0 {
			userLimit = u.SpeedLimit
			u.DynamicSpeedLimit = 0
			u.ExpireTime = 0
		} else {
			l.UserLimit.Delete(taguuid)
			return nil, true
		}
	} else {
		userLimit = determineSpeedLimit(u.SpeedLimit, u.DynamicSpeedLimit)
	}

	if noUDPsource || l.Nodetype == "hysteria2" || l.Nodetype == "tuic" {
		if reject := l.deviceTracker.TrackDevice(taguuid, ip, uid, deviceLimit); reject {
			return nil, true
		}
	}

	return l.getOrCreateSpeedLimiter(taguuid, nodeLimit, userLimit)
}

func (l *Limiter) getOrCreateSpeedLimiter(taguuid string, nodeLimit, userLimit int) (*rate.DynamicBucket, bool) {
	limit := int64(determineSpeedLimit(nodeLimit, userLimit)) * 1000000 / 8
	if limit <= 0 {
		return nil, false
	}

	if v, ok := l.SpeedLimiter.Load(taguuid); ok {
		return v.(*rate.DynamicBucket), false
	}

	d := rate.NewDynamicBucket(limit)
	l.SpeedLimiter.Store(taguuid, d)
	return d, false
}

func (l *Limiter) UpdateAliveList(newAlive map[int]int) {
	l.deviceTracker.UpdateAliveList(newAlive)
}

func (l *Limiter) GetOnlineDevice() (*[]panel.OnlineUser, error) {
	online := l.deviceTracker.GetOnlineDevices()
	return &online, nil
}
