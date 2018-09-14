package element

import (
	"os"

	"github.com/sirupsen/logrus"
)

// Start handles cluster events
func (a *Agent) Start(s chan os.Signal) {
	go func() {
		for range a.peerUpdateChan {
			if err := a.members.UpdateNode(nodeUpdateTimeout); err != nil {
				logrus.Errorf("error updating node metadata: %s", err)
			}
		}
	}()
}
