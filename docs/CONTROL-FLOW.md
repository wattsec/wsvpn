# Control flow

### Following is a high level overview of the control flow of WSVPN

```
                                         ┌───────────────┐
┌────────────┐  ┌─────────────┐          │ Authenticator │          ┌─────────────┐   ┌──────────────┐
│   Client   │─▷│  Connector  │──────────┼───────────────┼─────────▷│  Upgrader   │◁──│    Server    │
└────────────┘  ├─────────────┤          │     MTLS      │          ├─────────────┤   └──────────────┘
       ┌───────▶│   Adapter   │◀┐        │  HTTP Basic   │      ┌──▶│   Adapter   │─────────────────────┐
       │        └─────────────┘ │        └───────────────┘      │   └─────────────┘                     │
       │               │        │  ┌─────────────────────────┐  │          ▲                            │
       │               │        │  │       Connection        │  │          └───────────┐                │
       │               │        │  │ ┌──────────┐┌─────────┐ │  │                      ▼                │
       │               │        │  │ │   Data   ││         │ │  │        ┌──────────────────────────┐   │
       ▼               │        └─▶│ │          ││ Control │ │◀─┘        │         TUN/TAP          │   │
┌────────────┐         │           │ │  IP/Eth  ││         │ │           │                          │◁──┘
│  TUN/TAP   │◁────────┘           │ └──────────┘└─────────┘ │           │ One interface per client │
└────────────┘                     └─────────────────────────┘           └──────────────────────────┘



                                         ┌───────────────┐
┌────────────┐  ┌─────────────┐          │ Authenticator │          ┌─────────────┐   ┌──────────────┐
│   Client   │─▷│  Connector  │──────────┼───────────────┼─────────▷│  Upgrader   │◁──│    Server    │──┐
└────────────┘  ├─────────────┤          │     MTLS      │          ├─────────────┤   └──────────────┘  │
       ┌───────▶│   Adapter   │◀─┐       │  HTTP Basic   │       ┌─▶│   Adapter   │◀──┐       │         │
       │        └─────────────┘  │       └───────────────┘       │  └─────────────┘   │       │         │
       │               │         │  ┌─────────────────────────┐  │         │          │       └─────┐   │
       │               │         │  │       Connection        │  │  ┌──────┘          │             │   │
       │               │         │  │ ┌──────────┐┌─────────┐ │  │  │      ┌─────────────────────┐  │   │
       │               │         │  │ │   Data   ││         │ │  │  │      │   Packet handler    │  │   │
       ▼               │         └─▶│ │          ││ Control │ │◀─┘  │      │                     │◁─┘   │
┌────────────┐         │            │ │  IP/Eth  ││         │ │     │      │   Learning switch   │      │
│  TUN/TAP   │◁────────┘            │ └──────────┘└─────────┘ │     │      └─────────────────────┘      │
└────────────┘                      └─────────────────────────┘     │                 ▲                 │
                                                                    │                 │                 │
                                                                    │  ┌────────────────────────────┐   │
                                                                    │  │          TUN/TAP           │   │
                                                                    └─▶│                            │◁──┘
                                                                       │ One interface for everyone │
                                                                       └────────────────────────────┘
┌─────────────────────────────┐
│ Filled arrow = VPN packets  │
│ Hollow arrow = Control flow │
└─────────────────────────────┘
┌─────────────────────────┐┌─────────────────────────────────┐
│        WebSocket        ││          WebTransport           │
│                         ││                                 │
│  Data = Binary message  ││         Data = Datagram         │
│ Control = Text message  ││ Control = Bi-directional stream │
│   Ping = Ping message   ││  Ping = Bi-directional stream   │
└─────────────────────────┘└─────────────────────────────────┘
```