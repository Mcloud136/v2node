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
	mu                sync.RWMutex
}

// DeviceTracker 使用单个 RWMutex 保护所有设备追踪状态
// 替代原先 4 个 sync.Map，减少每连接 4-5 次 sync.Map 操作为 1 次 RLock/RUnlock
type DeviceTracker struct {
	mu          sync.RWMutex
	onlineIPs   map[string]int      // taguuid:ip → uid
	oldOnline   map[string]int      // ip → uid
	aliveCount  map[int]int         // uid → count
	userIPs     map[string]map[string]struct{} // taguuid → IP集合 (反向索引)
}

func NewDeviceTracker(aliveList map[int]int) *DeviceTracker {
	dt := &DeviceTracker{
		onlineIPs:  make(map[string]int),
		oldOnline:  make(map[string]int),
		aliveCount: make(map[int]int, len(aliveList)),
		userIPs:    make(map[string]map[string]struct{}),
	}
	for uid, count := range aliveList {
		dt.aliveCount[uid] = count
	}
	return dt
}

func (dt *DeviceTracker) TrackDevice(taguuid, ip string, uid, deviceLimit int) bool {
	var key strings.Builder
	key.Grow(len(taguuid) + 1 + len(ip))
	key.WriteString(taguuid)
	key.WriteByte(':')
	key.WriteString(ip)
	keyStr := key.String()

	// 快速路径：只读检查（RLock 允许并发读取）
	dt.mu.RLock()
	if existingUID, loaded := dt.onlineIPs[keyStr]; loaded {
		dt.mu.RUnlock()
		return existingUID != uid
	}
	dt.mu.RUnlock()

	// 慢路径：需要写入，升级为独占锁
	dt.mu.Lock()
	defer dt.mu.Unlock()

	// 再次检查（其他 goroutine 可能已插入）
	if existingUID, loaded := dt.onlineIPs[keyStr]; loaded {
		return existingUID != uid
	}

	dt.onlineIPs[keyStr] = uid

	// 维护反向索引
	if ips, ok := dt.userIPs[taguuid]; ok {
		ips[ip] = struct{}{}
	} else {
		dt.userIPs[taguuid] = map[string]struct{}{ip: {}}
	}

	if deviceLimit > 0 {
		count := dt.aliveCount[uid]
		if count >= deviceLimit {
			delete(dt.onlineIPs, keyStr)
			// 同时清理反向索引
			if ips, ok := dt.userIPs[taguuid]; ok {
				delete(ips, ip)
				if len(ips) == 0 {
					delete(dt.userIPs, taguuid)
				}
			}
			return true
		}
	}

	if oldUID, ok := dt.oldOnline[ip]; ok && oldUID == uid {
		delete(dt.oldOnline, ip)
	}

	return false
}

func (dt *DeviceTracker) UpdateAliveList(newAlive map[int]int) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	for uid := range dt.aliveCount {
		if _, exists := newAlive[uid]; !exists {
			delete(dt.aliveCount, uid)
		}
	}

	for uid, count := range newAlive {
		dt.aliveCount[uid] = count
	}
}

func (dt *DeviceTracker) GetOnlineDevices() []panel.OnlineUser {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	result := make([]panel.OnlineUser, 0, len(dt.onlineIPs))

	// 清空 oldOnline
	for k := range dt.oldOnline {
		delete(dt.oldOnline, k)
	}

	// 收集并迁移 onlineIPs 到 oldOnline
	for ipKey, uid := range dt.onlineIPs {
		colonIndex := strings.LastIndex(ipKey, ":")
		taguuid := ipKey[:colonIndex]
		ip := ipKey[colonIndex+1:]
		dt.oldOnline[ip] = uid
		result = append(result, panel.OnlineUser{UID: uid, IP: ip})

		// 清理反向索引
		if ips, ok := dt.userIPs[taguuid]; ok {
			delete(ips, ip)
			if len(ips) == 0 {
				delete(dt.userIPs, taguuid)
			}
		}
	}

	// 清空 onlineIPs
	for k := range dt.onlineIPs {
		delete(dt.onlineIPs, k)
	}

	return result
}

func (dt *DeviceTracker) DeleteUser(taguuid string) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	// 使用反向索引快速删除，避免遍历整个 Map
	if ips, ok := dt.userIPs[taguuid]; ok {
		for ip := range ips {
			var key strings.Builder
			key.Grow(len(taguuid) + 1 + len(ip))
			key.WriteString(taguuid)
			key.WriteByte(':')
			key.WriteString(ip)
			delete(dt.onlineIPs, key.String())
		}
		delete(dt.userIPs, taguuid)
	}
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
			u.mu.Lock()
			u.SpeedLimit = modified[i].SpeedLimit
			u.DeviceLimit = modified[i].DeviceLimit
			u.mu.Unlock()
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
		info.mu.Lock()
		info.DynamicSpeedLimit = limit
		info.ExpireTime = expire.Unix()
		info.mu.Unlock()
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
	u.mu.RLock()
	deviceLimit = u.DeviceLimit
	uid = u.UID
	speedLimit := u.SpeedLimit
	dynamicSpeedLimit := u.DynamicSpeedLimit
	expireTime := u.ExpireTime
	needReset := expireTime != 0 && expireTime < time.Now().Unix()
	u.mu.RUnlock()

	if needReset {
		// 使用写锁完成过期检查+重置，防止 TOCTOU 竞态覆盖新写入的值
		u.mu.Lock()
		if u.ExpireTime != 0 && u.ExpireTime < time.Now().Unix() {
			if u.SpeedLimit != 0 {
				userLimit = u.SpeedLimit
				u.DynamicSpeedLimit = 0
				u.ExpireTime = 0
			} else {
				u.mu.Unlock()
				l.UserLimit.Delete(taguuid)
				return nil, true
			}
		} else {
			// 其他 goroutine 已更新过期时间，重新读取
			userLimit = determineSpeedLimit(u.SpeedLimit, u.DynamicSpeedLimit)
		}
		u.mu.Unlock()
	} else {
		userLimit = determineSpeedLimit(speedLimit, dynamicSpeedLimit)
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
	actual, _ := l.SpeedLimiter.LoadOrStore(taguuid, d)
	return actual.(*rate.DynamicBucket), false
}

func (l *Limiter) UpdateAliveList(newAlive map[int]int) {
	l.deviceTracker.UpdateAliveList(newAlive)
}

func (l *Limiter) GetOnlineDevice() (*[]panel.OnlineUser, error) {
	online := l.deviceTracker.GetOnlineDevices()
	return &online, nil
}
