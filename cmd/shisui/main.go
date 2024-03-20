package main

import (
	"crypto/ecdsa"
	"fmt"
	"net"
	"strings"

	"os"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/internal/flags"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/discover/portalwire"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/portalnetwork/history"
	"github.com/ethereum/go-ethereum/portalnetwork/storage/sqlite"
	"github.com/urfave/cli/v2"
)

type Config struct {
	Protocol     *discover.PortalProtocolConfig
	PrivateKey   *ecdsa.PrivateKey
	RpcAddr      string
	DataDir      string
	DataCapacity uint64
	LogLevel     int
}

var app = flags.NewApp("the go-portal-network command line interface")

var (
	portalProtocolFlags = []cli.Flag{
		utils.PortalUDPListenAddrFlag,
		utils.PortalUDPPortFlag,
	}
	historyRpcFlags = []cli.Flag{
		utils.PortalRPCListenAddrFlag,
		utils.PortalRPCPortFlag,
		utils.PortalDataDirFlag,
		utils.PortalDataCapacityFlag,
		utils.PortalLogLevelFlag,
	}
)

func init() {
	app.Action = shisui
	app.Flags = flags.Merge(portalProtocolFlags, historyRpcFlags)
	flags.AutoEnvVars(app.Flags, "SHISUI")
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func shisui(ctx *cli.Context) error {
	config, err := getPortalHistoryConfig(ctx)
	if err != nil {
		return nil
	}

	glogger := log.NewGlogHandler(log.NewTerminalHandler(os.Stderr, true))
	slogVerbosity := log.FromLegacyLevel(config.LogLevel)
	glogger.Verbosity(slogVerbosity)
	log.SetDefault(log.NewLogger(glogger))

	nodeId := enode.PubkeyToIDV4(&config.PrivateKey.PublicKey)
	contentStorage, err := sqlite.NewContentStorage(config.DataCapacity, nodeId, config.DataDir)
	if err != nil {
		return err
	}

	contentQueue := make(chan *discover.ContentElement, 50)

	protocol, err := discover.NewPortalProtocol(config.Protocol, string(portalwire.HistoryNetwork), config.PrivateKey, contentStorage, contentQueue)

	if err != nil {
		return err
	}

	accumulator, err := history.NewMasterAccumulator()
	if err != nil {
		return err
	}

	historyNetwork := history.NewHistoryNetwork(protocol, &accumulator)
	err = historyNetwork.Start()
	if err != nil {
		return err
	}
	defer historyNetwork.Stop()

	discover.StartHistoryRpcServer(protocol, config.RpcAddr)

	return nil
}

func getPortalHistoryConfig(ctx *cli.Context) (*Config, error) {
	config := &Config{
		Protocol: discover.DefaultPortalProtocolConfig(),
	}
	err := setPrivateKey(ctx, config)
	if err != nil {
		return config, err
	}

	httpAddr := ctx.String(utils.PortalRPCListenAddrFlag.Name)
	httpPort := ctx.String(utils.PortalRPCPortFlag.Name)
	config.RpcAddr = net.JoinHostPort(httpAddr, httpPort)
	config.DataDir = ctx.String(utils.PortalDataDirFlag.Name)
	config.DataCapacity = ctx.Uint64(utils.PortalDataCapacityFlag.Name)
	config.LogLevel = ctx.Int(utils.PortalLogLevelFlag.Name)
	port := ctx.String(utils.PortalUDPPortFlag.Name)
	if !strings.HasPrefix(port, ":") {
		config.Protocol.ListenAddr = ":" + port
	} else {
		config.Protocol.ListenAddr = port
	}

	if ctx.IsSet(utils.PortalUDPListenAddrFlag.Name) {
		ip := ctx.String(utils.PortalUDPListenAddrFlag.Name)
		netIp := net.ParseIP(ip)
		if netIp == nil {
			return config, fmt.Errorf("invalid ip addr: %s", ip)
		}
		config.Protocol.NodeIP = netIp
	}

	if ctx.IsSet(utils.PortalBootNodesFlag.Name) {
		for _, node := range ctx.StringSlice(utils.PortalBootNodesFlag.Name) {
			bootNode := new(enode.Node)
			err = bootNode.UnmarshalText([]byte(node))
			if err != nil {
				return config, err
			}
			config.Protocol.BootstrapNodes = append(config.Protocol.BootstrapNodes, bootNode)
		}
	}
	return config, nil
}

func setPrivateKey(ctx *cli.Context, config *Config) error {
	var privateKey *ecdsa.PrivateKey
	var err error
	if ctx.IsSet(utils.PortalPrivateKeyFlag.Name) {
		keyStr := ctx.String(utils.PortalPrivateKeyFlag.Name)
		keyBytes, err := hexutil.Decode(keyStr)
		if err != nil {
			return err
		}
		privateKey, err = crypto.ToECDSA(keyBytes)
		if err != nil {
			return err
		}
	} else {
		privateKey, err = crypto.GenerateKey()
		if err != nil {
			return err
		}
	}
	config.PrivateKey = privateKey
	return nil
}