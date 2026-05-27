package panel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack/v5"
)

type OnlineUser struct {
	UID int
	IP  string
}

type UserInfo struct {
	Id          int    `json:"id" msgpack:"id"`
	Uuid        string `json:"uuid" msgpack:"uuid"`
	SpeedLimit  int    `json:"speed_limit" msgpack:"speed_limit"`
	DeviceLimit int    `json:"device_limit" msgpack:"device_limit"`
}

type UserListBody struct {
	Users []UserInfo `json:"users" msgpack:"users"`
}

type AliveMap struct {
	Alive map[int]int `json:"alive"`
}

// GetUserList will pull user from v2board
func (c *Client) GetUserList(ctx context.Context) ([]UserInfo, error) {
	const path = "/api/v1/server/UniProxy/user"
	r, err := c.client.R().
		SetContext(ctx).
		SetHeader("If-None-Match", c.userEtag).
		SetHeader("X-Response-Format", "msgpack").
		SetDoNotParseResponse(true).
		Get(path)
	if err != nil {
		return nil, err
	}
	if r == nil || r.RawResponse == nil {
		return nil, fmt.Errorf("received nil response or raw response")
	}
	defer r.RawResponse.Body.Close()

	if r.StatusCode() == 304 {
		return nil, nil
	}
	body, err := io.ReadAll(r.RawResponse.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body error: %w", err)
	}
	contentType := r.Header().Get("Content-Type")
	if strings.Contains(contentType, "application/x-msgpack") {
		var userlist UserListBody
		if err := msgpack.Unmarshal(body, &userlist); err != nil {
			return nil, fmt.Errorf("decode user list error: %w", err)
		}
		c.userEtag = r.Header().Get("ETag")
		return userlist.Users, nil
	}
	var userlist UserListBody
	if err := json.Unmarshal(body, &userlist); err != nil {
		return nil, fmt.Errorf("decode user list error: %w", err)
	}
	c.userEtag = r.Header().Get("ETag")
	return userlist.Users, nil
}

// GetUserAlive will fetch the alive_ip count for users
func (c *Client) GetUserAlive(ctx context.Context) (map[int]int, error) {
	c.AliveMap = &AliveMap{}
	const path = "/api/v1/server/UniProxy/alivelist"
	r, err := c.client.R().
		SetContext(ctx).
		ForceContentType("application/json").
		Get(path)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		c.AliveMap.Alive = make(map[int]int)
		return c.AliveMap.Alive, nil
	}
	if r == nil || r.RawResponse == nil || r.StatusCode() >= 399 {
		c.AliveMap.Alive = make(map[int]int)
		return c.AliveMap.Alive, nil
	}
	defer r.RawResponse.Body.Close()
	if err := json.Unmarshal(r.Body(), c.AliveMap); err != nil {
		logrus.WithField("err", err).Error("unmarshal user alive list error")
		c.AliveMap.Alive = make(map[int]int)
	}

	return c.AliveMap.Alive, nil
}

type UserTraffic struct {
	UID      int
	Upload   int64
	Download int64
}

// ReportUserTraffic reports the user traffic
func (c *Client) ReportUserTraffic(ctx context.Context, userTraffic []UserTraffic) error {
	// 复用缓冲区，避免每次上报分配新 map
	if c.reportBuffer == nil {
		c.reportBuffer = make(map[int][]int64, len(userTraffic))
	}
	// 清空旧数据
	for k := range c.reportBuffer {
		delete(c.reportBuffer, k)
	}
	for i := range userTraffic {
		c.reportBuffer[userTraffic[i].UID] = []int64{userTraffic[i].Upload, userTraffic[i].Download}
	}
	const path = "/api/v1/server/UniProxy/push"
	_, err := c.client.R().
		SetContext(ctx).
		SetBody(c.reportBuffer).
		ForceContentType("application/json").
		Post(path)
	return err
}

func (c *Client) ReportNodeOnlineUsers(ctx context.Context, data *map[int][]string) error {
	const path = "/api/v1/server/UniProxy/alive"
	_, err := c.client.R().
		SetContext(ctx).
		SetBody(data).
		ForceContentType("application/json").
		Post(path)

	if err != nil {
		return err
	}

	return nil
}
