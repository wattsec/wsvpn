package sockets

import (
	"log"
	"sync"
	"time"

	"github.com/Doridian/wsvpn/shared"
	"github.com/Doridian/wsvpn/shared/commands"
	"github.com/Doridian/wsvpn/shared/sockets/adapters"
	"github.com/songgao/water"
)

const UndeterminedProtocolVersion = 0
const featureFieldMinProtocol = 12

type Socket struct {
	AssignedIP shared.IPv4

	lastFragmentId        uint32
	lastFragmentCleanup   time.Time
	defragBuffer          map[uint32]*fragmentsInfo
	defragLock            *sync.Mutex
	fragmentCleanupTicker *time.Ticker
	fragmentationEnabled  bool

	compressionEnabled bool

	remoteProtocolVersion int

	adapter          adapters.SocketAdapter
	iface            *water.Interface
	ifaceManaged     bool
	wg               *sync.WaitGroup
	readyWait        *sync.Cond
	handlers         map[string]CommandHandler
	closechan        chan bool
	closechanopen    bool
	mac              shared.MacAddr
	packetBufferSize int
	packetHandler    PacketHandler
	log              *log.Logger
	pingInterval     time.Duration
	pingTimeout      time.Duration
	isReady          bool
	isClosing        bool
	closeLock        *sync.Mutex

	localFeatures  map[commands.Feature]bool
	remoteFeatures map[commands.Feature]bool
	usedFeatures   map[commands.Feature]bool
}

func MakeSocket(logger *log.Logger, adapter adapters.SocketAdapter, iface *water.Interface, ifaceManaged bool) *Socket {
	return &Socket{
		AssignedIP: shared.DefaultIPv4,

		adapter:               adapter,
		iface:                 iface,
		ifaceManaged:          ifaceManaged,
		wg:                    &sync.WaitGroup{},
		readyWait:             shared.MakeSimpleCond(),
		handlers:              make(map[commands.CommandName]CommandHandler),
		closechan:             make(chan bool),
		closechanopen:         true,
		mac:                   shared.DefaultMac,
		remoteProtocolVersion: UndeterminedProtocolVersion,
		packetBufferSize:      2000,
		log:                   logger,
		isReady:               false,
		isClosing:             false,
		closeLock:             &sync.Mutex{},

		lastFragmentId:        0,
		defragBuffer:          make(map[uint32]*fragmentsInfo),
		defragLock:            &sync.Mutex{},
		lastFragmentCleanup:   time.Now(),
		fragmentCleanupTicker: time.NewTicker(fragmentExpiryTime),
		fragmentationEnabled:  false,

		compressionEnabled: false,

		localFeatures:  make(map[commands.Feature]bool, 0),
		remoteFeatures: make(map[commands.Feature]bool, 0),
		usedFeatures:   make(map[commands.Feature]bool),
	}
}

func (s *Socket) ConfigurePing(pingInterval time.Duration, pingTimeout time.Duration) {
	s.pingInterval = pingInterval
	s.pingTimeout = pingTimeout
}

func (s *Socket) SetLocalFeature(feature commands.Feature, enabled bool) {
	if !enabled {
		delete(s.localFeatures, feature)
		return
	}
	s.localFeatures[feature] = true
}

func (s *Socket) IsLocalFeature(feature commands.Feature) bool {
	return s.localFeatures[feature]
}

func (s *Socket) SetPacketHandler(packetHandler PacketHandler) {
	s.packetHandler = packetHandler
}

func (s *Socket) HandleInitPacketFragmentation(enabled bool) {
	if s.remoteProtocolVersion >= featureFieldMinProtocol {
		return
	}

	s.SetLocalFeature(commands.FEATURE_FRAGMENTATION, true)
	if enabled {
		s.remoteFeatures[commands.FEATURE_FRAGMENTATION] = true
	} else {
		delete(s.remoteFeatures, commands.FEATURE_FRAGMENTATION)
	}

	s.featureCheck()
}

func (s *Socket) featureCheck() {
	if s.remoteProtocolVersion == UndeterminedProtocolVersion {
		return
	}

	s.usedFeatures = make(map[commands.Feature]bool)
	for feat, en := range s.localFeatures {
		if !en {
			continue
		}
		if s.remoteFeatures[feat] {
			s.usedFeatures[feat] = true
		}
		log.Printf("F = %s, L = %v, R = %v, U = %v", feat, en, s.remoteFeatures[feat], s.usedFeatures[feat])
	}

	if s.remoteProtocolVersion >= fragmentationMinProtocol && s.remoteProtocolVersion < fragmentationNegotiatedMinProtocol {
		s.fragmentationEnabled = true
	} else if s.remoteProtocolVersion >= fragmentationNegotiatedMinProtocol && s.remoteProtocolVersion < featureFieldMinProtocol {
		s.fragmentationEnabled = s.localFeatures[commands.FEATURE_FRAGMENTATION]
	} else {
		s.fragmentationEnabled = s.usedFeatures[commands.FEATURE_FRAGMENTATION]
	}

	s.compressionEnabled = s.usedFeatures[commands.FEATURE_COMPRESSION]

	s.log.Printf("Setting fragmentation: %s", shared.BoolToEnabled(s.fragmentationEnabled))
	s.log.Printf("Setting compression: %s", shared.BoolToEnabled(s.compressionEnabled))
}

func (s *Socket) Wait() {
	s.wg.Wait()
}

func (s *Socket) WaitReady() {
	for !s.isReady {
		s.readyWait.L.Lock()
		s.readyWait.Wait()
		s.readyWait.L.Unlock()
	}
}

func (s *Socket) closeDone() {
	s.wg.Done()
	s.Close()
}

func (s *Socket) CloseError(err error) {
	if !s.isClosing {
		s.isClosing = true
		s.log.Printf("Closing socket: %v", err)
		s.SendMessage("error", err.Error())
	}
	s.Close()
}

func (s *Socket) setReady() {
	s.isReady = true
	s.readyWait.Broadcast()
}

func (s *Socket) Close() {
	s.closeLock.Lock()
	defer s.closeLock.Unlock()

	s.adapter.Close()
	if s.iface != nil && s.ifaceManaged {
		s.iface.Close()
	}

	if s.closechanopen {
		s.closechanopen = false
		close(s.closechan)
	}

	if s.packetHandler != nil {
		s.packetHandler.UnregisterSocket(s)
	}

	s.setReady()

	s.fragmentCleanupTicker.Stop()
}

func (s *Socket) Serve() {
	s.registerDefaultCommandHandlers()

	if s.packetHandler != nil {
		s.packetHandler.RegisterSocket(s)
	}

	s.adapter.SetDataMessageHandler(s.dataMessageHandler)

	s.registerControlMessageHandler()

	s.installPingPongHandlers()

	s.wg.Add(1)
	go func() {
		defer s.closeDone()
		err, unexpected := s.adapter.Serve()
		if unexpected {
			s.log.Printf("Adapter ERROR: %v", err)
		}
	}()

	s.adapter.WaitReady()

	go s.cleanupFragmentsLoop()

	s.tryServeIfaceRead()

	go s.sendDefaultWelcome()
}
