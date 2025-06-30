package bft

import (
	"fmt"
	"time"
)

type InsiderAttack struct {
	StopChannel chan struct{}
}

func NewInsiderAttackInstance() *InsiderAttack {
	return &InsiderAttack{
		StopChannel: make(chan struct{}, 1),
	}
}

// StartAttackLeader zhf add code 攻击 leader
func (insiderAttack *InsiderAttack) StartAttackLeader(controller *Controller, duration *time.Duration) {
	go func() {
	Loop:
		for {
			select {
			case <-insiderAttack.StopChannel:
				break Loop
			default:
				if yes, leader := controller.iAmTheLeader(); !yes {
					fmt.Printf("current leader: %d", leader)
					controller.SendConsensus(leader, controller.RecentlySendMsg)
				}
				if duration != nil {
					time.Sleep(*duration)
				}
			}
		}
	}()
}

func (insiderAttack *InsiderAttack) StopAttackLeader() {
	insiderAttack.StopChannel <- struct{}{}
}
