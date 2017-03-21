package msocks

import "time"

func recvWithTimeout(ch chan uint32, t time.Duration) (errno uint32) {
	var ok bool
	ch_timeout := time.After(t)
	select {
	case errno, ok = <-ch:
		if !ok {
			return ERR_CLOSED
		}
	case <-ch_timeout:
		return ERR_TIMEOUT
	}
	return
}

func (c *Conn) WaitForConn() (err error) {
	c.chSynResult = make(chan uint32, 0)

	fb := &FrameSyn{StreamId:c.streamId, Address:c.Address}
	err = c.session.SendFrame(fb)
	if err != nil {
		log.Errorf("%s", err)
		c.final()
		return
	}

	errno := recvWithTimeout(c.chSynResult, DIAL_TIMEOUT * time.Second)
	if errno != ERR_NONE {
		log.Errorf("remote connect %s failed for %d.", c.String(), errno)
		c.final()
	} else {
		log.Noticef("%s connected: %s.", c.Address.String(), c.String())
	}

	c.chSynResult = nil
	return
}