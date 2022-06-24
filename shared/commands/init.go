package commands

const InitCommandName CommandName = "init"

type InitParameters struct {
	Mode      InterfaceMode     `json:"mode"`
	IpAddress IpAddressWithCIDR `json:"ip_address"`
	MTU       int               `json:"mtu"`
}

func (c *InitParameters) MakeCommand(id string) (*OutgoingCommand, error) {
	return makeCommand(InitCommandName, id, c)
}