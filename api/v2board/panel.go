package panel

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/go-resty/resty/v2"
	"github.com/wyx2685/v2node/conf"
)

// Panel is the interface for different panel's api.

type Client struct {
	client           *resty.Client
	APIHost          string
	Token            string
	NodeId           int
	nodeEtag         string
	userEtag         string
	responseBodyHash string
	UserList         *UserListBody
	AliveMap         *AliveMap
	reportBuffer     map[int][]int64 // 复用的流量上报缓冲区
}

func New(c *conf.NodeConfig) (*Client, error) {
	client := resty.New()
	// 优化 HTTP 连接池配置，保持长连接复用
	client.SetTransport(&http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	})
	retryCount := conf.DefaultNodeRetryCount
	if c.RetryCount != nil {
		retryCount = *c.RetryCount
	}
	client.SetRetryCount(retryCount)

	// 使用strings.Builder优化User-Agent字符串构造
	var userAgent strings.Builder
	userAgent.Grow(len("v2node go-resty/ (https://github.com/go-resty/resty)") + len(resty.Version))
	userAgent.WriteString("v2node go-resty/")
	userAgent.WriteString(resty.Version)
	userAgent.WriteString(" (https://github.com/go-resty/resty)")
	client.SetHeader("User-Agent", userAgent.String())

	if c.Timeout > 0 {
		client.SetTimeout(time.Duration(c.Timeout) * time.Second)
	} else {
		client.SetTimeout(time.Duration(conf.DefaultNodeTimeout) * time.Second)
	}
	client.OnError(func(req *resty.Request, err error) {
		var v *resty.ResponseError
		if errors.As(err, &v) {
			// v.Response contains the last response from the server
			// v.Err contains the original error
			logrus.Error(v.Err)
		}
	})
	client.SetBaseURL(c.APIHost)
	// set params
	client.SetQueryParams(map[string]string{
		"node_type": "v2node",
		"node_id":   strconv.Itoa(c.NodeID),
	})
	// API Token 通过请求头传递，避免暴露在 URL 中被日志记录
	client.SetHeader("Authorization", "Bearer "+c.Key)
	return &Client{
		client:   client,
		Token:    c.Key,
		APIHost:  c.APIHost,
		NodeId:   c.NodeID,
		UserList: &UserListBody{},
		AliveMap: &AliveMap{},
	}, nil
}
