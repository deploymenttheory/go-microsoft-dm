package mdm

import (
	"context"
	"strconv"
	"strings"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

// recordPackageOne stores the device facts from package 1 and calls the
// PackageOne hook once.
func (s *Service) recordPackageOne(ctx context.Context, sess *Session, req *syncml.Message) error {
	facts := &sess.Facts
	facts.DevInfo = syncml.DevInfo(req)
	facts.LoginStatus = syncml.LoginStatus(req)
	for _, a := range req.Body.Alerts() {
		if a.Code() != syncml.AlertClientEvent {
			continue
		}
		for _, it := range a.Items {
			if it.Meta == nil {
				continue
			}
			switch it.Meta.Type {
			case syncml.AlertTypeSyncType:
				facts.SyncType = it.Data.Text()
			case syncml.AlertTypeDevicePrepSync:
				facts.DevicePrepSync = it.Data.Text()
			}
		}
	}
	sess.gotPackageOne = true
	if s.cfg.Hooks != nil {
		if err := s.cfg.Hooks.PackageOne(ctx, sess.DeviceID, sess.SessionID, *facts); err != nil {
			return err
		}
	}
	if sess.Closed {
		return nil // the session is ending; do not queue new reads
	}
	// Device details are read once; push channel/status are read every session.
	return s.queueFirstSessionReads(ctx, sess)
}

// handleAlert acknowledges a client alert and routes the ones the engine
// does not consume to the hooks. LoginStatus, SyncType and DevicePrepSync
// (1224) become Facts in recordPackageOne and are only acknowledged here.
func (s *Service) handleAlert(ctx context.Context, sess *Session, a *syncml.Alert, resp *syncml.Message, ids *syncml.CmdIDs) error {
	code := a.Code()
	resp.Body.Commands = append(resp.Body.Commands, &syncml.Status{
		CmdID: ids.Next(), MsgRef: strconv.Itoa(sess.ClientMsgID), CmdRef: a.ID(), Cmd: syncml.CmdAlert,
		Data: syncml.Data{Value: syncml.StatusOK.Wire()},
	})
	switch code {
	case syncml.AlertNextMessage, syncml.AlertServerInitiated, syncml.AlertClientInitiated:
		return nil
	case syncml.AlertSessionAbort:
		sess.Closed = true
		return nil
	case syncml.AlertNoEndOfData:
		if sess.Assembler != nil {
			sess.Assembler.Reset()
		}
		sess.AssemblingCommand = ""
		return nil
	}
	if s.cfg.Hooks == nil {
		return nil
	}
	for _, it := range a.Items {
		e := Event{DeviceID: sess.DeviceID, SessionID: sess.SessionID, Alert: int(code), Data: it.Data.Text()}
		if it.Meta != nil {
			e.Type, e.Mark = it.Meta.Type, it.Meta.Mark
		}
		e.Source = it.Source
		switch code {
		case syncml.AlertGeneric:
			if err := s.cfg.Hooks.GenericAlert(ctx, e); err != nil {
				return err
			}
			if isUnenrollAlert(it) {
				sess.Closed, sess.unenroll = true, true
			}
		case syncml.AlertClientEvent:
			if isConsumedEvent(it) {
				continue
			}
			if err := s.cfg.Hooks.ClientEvent(ctx, e); err != nil {
				return err
			}
		}
	}
	return nil
}

// isConsumedEvent reports whether a 1224 item is one the engine records as a
// fact rather than delivering to ClientEvent.
func isConsumedEvent(it syncml.Item) bool {
	if it.Meta == nil {
		return false
	}
	switch it.Meta.Type {
	case syncml.AlertTypeLoginStatus, syncml.AlertTypeSyncType, syncml.AlertTypeDevicePrepSync:
		return true
	}
	return false
}

// isUnenrollAlert reports whether a generic alert item is the
// user-initiated unenroll request.
func isUnenrollAlert(it syncml.Item) bool {
	return it.Meta != nil && strings.EqualFold(it.Meta.Type, syncml.AlertTypeUnenrollmentUserRequest)
}
