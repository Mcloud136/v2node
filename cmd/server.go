package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/wyx2685/v2node/conf"
	"github.com/wyx2685/v2node/core"
	"github.com/wyx2685/v2node/limiter"
	"github.com/wyx2685/v2node/node"
)

var (
	config string
	watch  bool
)

var serverCommand = cobra.Command{
	Use:   "server",
	Short: "Run v2node server",
	RunE:  serverHandle,
	Args:  cobra.NoArgs,
}

func init() {
	serverCommand.PersistentFlags().
		StringVarP(&config, "config", "c",
			"/etc/v2node/config.json", "config file path")
	serverCommand.PersistentFlags().
		BoolVarP(&watch, "watch", "w",
			true, "watch file path change")
	command.AddCommand(&serverCommand)
}

func setLogLevel(level string) {
	switch level {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn", "warning":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	}
}

func setLogOutput(output string) (*os.File, error) {
	if output == "" {
		return nil, nil
	}
	f, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	if oldWriter, ok := log.StandardLogger().Out.(*os.File); ok && oldWriter != os.Stdout && oldWriter != os.Stderr {
		oldWriter.Close()
	}
	log.SetOutput(f)
	return f, nil
}

func validateConfig(c *conf.Conf) error {
	if len(c.NodeConfigs) == 0 {
		return fmt.Errorf("no node configs found")
	}
	for i, nodeConf := range c.NodeConfigs {
		if nodeConf.APIHost == "" {
			return fmt.Errorf("node[%d]: ApiHost is required", i)
		}
		if nodeConf.NodeID <= 0 {
			return fmt.Errorf("node[%d]: NodeID must be positive", i)
		}
		if nodeConf.Key == "" {
			return fmt.Errorf("node[%d]: ApiKey is required", i)
		}
		if nodeConf.Timeout <= 0 {
			nodeConf.Timeout = conf.DefaultNodeTimeout
		}
	}
	if c.PprofPort < 0 || c.PprofPort > 65535 {
		return fmt.Errorf("PprofPort must be between 0 and 65535")
	}
	return nil
}

func isPortAvailable(port int) bool {
	if port <= 0 {
		return true
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

type ServerResources struct {
	pprofServer *http.Server
	logFile     *os.File
}

func (sr *ServerResources) Close() {
	if sr.pprofServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := sr.pprofServer.Shutdown(ctx); err != nil {
			log.WithField("err", err).Warn("pprof server shutdown failed")
		}
		log.Info("pprof server stopped")
	}
	if sr.logFile != nil {
		if err := sr.logFile.Close(); err != nil {
			log.WithField("err", err).Warn("log file close failed")
		}
	}
}

func serverHandle(_ *cobra.Command, _ []string) error {
	showVersion()

	var resources ServerResources
	defer resources.Close()

	c := conf.New()
	err := c.LoadFromPath(config)
	log.SetFormatter(&log.TextFormatter{
		DisableTimestamp: true,
		DisableQuote:     true,
		PadLevelText:     false,
	})
	if err != nil {
		log.WithField("err", err).Error("Load config file failed")
		return err
	}

	if err := validateConfig(c); err != nil {
		log.WithField("err", err).Error("Config validation failed")
		return err
	}

	setLogLevel(c.LogConfig.Level)
	if resources.logFile, err = setLogOutput(c.LogConfig.Output); err != nil {
		log.WithField("err", err).Error("Open log file failed, using stdout instead")
	}

	if c.PprofPort != 0 {
		if !isPortAvailable(c.PprofPort) {
			log.WithField("port", c.PprofPort).Error("pprof port is not available")
			return fmt.Errorf("pprof port %d not available", c.PprofPort)
		}
		resources.pprofServer = &http.Server{
			Addr: fmt.Sprintf("127.0.0.1:%d", c.PprofPort),
		}
		go func() {
			log.Infof("Starting pprof server on :%d", c.PprofPort)
			if err := resources.pprofServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.WithField("err", err).Error("pprof server failed")
			}
		}()
	}

	limiter.Init()

	nodes, err := node.New(c.NodeConfigs)
	if err != nil {
		log.WithField("err", err).Error("Get node info failed")
		return err
	}
	log.Info("Got nodes info from server")

	var reloadCh = make(chan struct{}, 1)
	v2core := core.New(c)
	v2core.ReloadCh = reloadCh
	err = v2core.Start(nodes.NodeInfos)
	if err != nil {
		log.WithField("err", err).Error("Start core failed")
		return err
	}
	defer v2core.Close()

	err = nodes.Start(c.NodeConfigs, v2core)
	if err != nil {
		log.WithField("err", err).Error("Run nodes failed")
		return err
	}
	log.Info("Nodes started")

	if watch {
		err = c.Watch(config, func() {
			select {
			case reloadCh <- struct{}{}:
			default:
			}
		})
		if err != nil {
			log.WithField("err", err).Error("start watch failed")
			return err
		}
	}

	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-osSignals:
			log.Info("Received shutdown signal, closing...")
			return nil
		case <-reloadCh:
			log.Info("Received reload signal, reloading configuration...")
			if err := reload(config, &nodes, &v2core, &resources); err != nil {
				log.WithField("err", err).Error("Reload failed, keeping current configuration")
			} else {
				log.Info("Reload successful")
			}
		}
	}
}

func reload(configPath string, nodes **node.Node, v2core **core.V2Core, resources *ServerResources) error {
	newConf := conf.New()
	if err := newConf.LoadFromPath(configPath); err != nil {
		log.WithField("err", err).Error("Failed to load new config")
		return err
	}

	if err := validateConfig(newConf); err != nil {
		log.WithField("err", err).Error("New config validation failed")
		return err
	}

	if newConf.PprofPort != 0 && (resources.pprofServer == nil || fmt.Sprintf("127.0.0.1:%d", newConf.PprofPort) != resources.pprofServer.Addr) {
		if !isPortAvailable(newConf.PprofPort) {
			log.WithField("port", newConf.PprofPort).Error("New pprof port is not available")
			return fmt.Errorf("pprof port %d not available", newConf.PprofPort)
		}
	}

	var oldReloadCh chan struct{}
	if *v2core != nil {
		oldReloadCh = (*v2core).ReloadCh
	}

	// 先构建新配置，成功后再关闭旧配置（保证可回滚）
	newNodes, err := node.New(newConf.NodeConfigs)
	if err != nil {
		log.WithField("err", err).Error("Failed to create new nodes, keeping current configuration")
		return err
	}

	newCore := core.New(newConf)
	newCore.ReloadCh = oldReloadCh
	if err := newCore.Start(newNodes.NodeInfos); err != nil {
		log.WithField("err", err).Error("Failed to start new core, keeping current configuration")
		return err
	}

	if err := newNodes.Start(newConf.NodeConfigs, newCore); err != nil {
		log.WithField("err", err).Error("Failed to start new nodes, rolling back")
		newCore.Close()
		return err
	}

	// 新配置启动成功，关闭旧配置
	if err := (*nodes).Close(); err != nil {
		log.WithField("err", err).Warn("Failed to close old nodes (non-fatal)")
	}
	if err := (*v2core).Close(); err != nil {
		log.WithField("err", err).Warn("Failed to close old core (non-fatal)")
	}

	if newConf.LogConfig.Level != log.GetLevel().String() {
		setLogLevel(newConf.LogConfig.Level)
	}
	if newConf.LogConfig.Output != "" {
		if newFile, err := setLogOutput(newConf.LogConfig.Output); err == nil {
			if resources.logFile != nil {
				resources.logFile.Close()
			}
			resources.logFile = newFile
		}
	}

	if newConf.PprofPort != 0 && (resources.pprofServer == nil || fmt.Sprintf("127.0.0.1:%d", newConf.PprofPort) != resources.pprofServer.Addr) {
		if resources.pprofServer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			resources.pprofServer.Shutdown(ctx)
		}
		resources.pprofServer = &http.Server{
			Addr: fmt.Sprintf("127.0.0.1:%d", newConf.PprofPort),
		}
		go func() {
			log.Infof("Starting pprof server on :%d", newConf.PprofPort)
			if err := resources.pprofServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.WithField("err", err).Error("pprof server failed")
			}
		}()
	}

	*nodes = newNodes
	*v2core = newCore

	return nil
}
