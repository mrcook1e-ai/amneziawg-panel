package awg

import (
	"errors"
	"fmt"
)

var ErrPersistedProfileIncompatible = errors.New("persisted profile incompatible with configured network mapping")

type PersistedProfileError struct {
	ProfileID string
	ClientID  string
	Rule      string
}

func (e *PersistedProfileError) Error() string {
	if e.ClientID != "" {
		return fmt.Sprintf("profile %q client %q violates %s", e.ProfileID, e.ClientID, e.Rule)
	}
	return fmt.Sprintf("profile %q violates %s", e.ProfileID, e.Rule)
}

func (e *PersistedProfileError) Is(target error) bool {
	return target == ErrPersistedProfileIncompatible
}

func (m *Manager) validatePersistedMappings(c *Config) error {
	for profileID, profile := range c.Profiles {
		if !m.portIPAM.Valid(profile.Port) {
			return &PersistedProfileError{ProfileID: profileID, Rule: "port_range"}
		}
		if profile.Iface != m.portIPAM.IfaceFor(profile.Port) {
			return &PersistedProfileError{ProfileID: profileID, Rule: "interface_mapping"}
		}
		ipam := m.ipamForPort(profile.Port)
		if profile.Address != ipam.ServerIP() {
			return &PersistedProfileError{ProfileID: profileID, Rule: "server_subnet"}
		}
		for clientID, client := range c.Clients {
			if client.ProfileID == profileID && !ipam.Valid(client.Address) {
				return &PersistedProfileError{ProfileID: profileID, ClientID: clientID, Rule: "client_subnet"}
			}
		}
	}
	return nil
}
