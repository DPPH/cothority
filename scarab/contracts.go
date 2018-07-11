package scarab

import (
	"errors"

	"github.com/dedis/cothority"
	ol "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
)

// ContractWriteID references a write contract system-wide.
var ContractWriteID = "scarabWrite"

// ContractWrite is used to store a secret in OmniLedger, so that an
// authorized reader can retrieve it by creating a Read-instance.
//
// Accepted Instructions:
//  - spawn:scarabWrite creates a new write-request. TODO: verify the LTS exists
//  - spawn:scarabRead creates a new read-request for this write-request.
func (s *Service) ContractWrite(cdb ol.CollectionView, inst ol.Instruction, c []ol.Coin) ([]ol.StateChange, []ol.Coin, error) {
	switch {
	case inst.Spawn != nil:
		var sc ol.StateChanges
		nc := c
		switch inst.Spawn.ContractID {
		case ContractWriteID:
			w := inst.Spawn.Args.Search("write")
			if w == nil || len(w) == 0 {
				return nil, nil, errors.New("need a write request in 'write' argument")
			}
			var wr Write
			err := protobuf.DecodeWithConstructors(w, &wr, network.DefaultConstructors(cothority.Suite))
			if err != nil {
				return nil, nil, errors.New("couldn't unmarshal write: " + err.Error())
			}
			if err = wr.CheckProof(cothority.Suite, inst.InstanceID.DarcID); err != nil {
				return nil, nil, errors.New("proof of write failed: " + err.Error())
			}
			log.Lvlf2("Successfully verified write request and will store in %x", inst.DeriveID("write"))
			sc = append(sc, ol.NewStateChange(ol.Create, inst.DeriveID("write"), ContractWriteID, w))
		case ContractReadID:
			var scs ol.StateChanges
			var err error
			scs, nc, err = s.ContractRead(cdb, inst, c)
			if err != nil {
				return nil, nil, err
			}
			sc = append(sc, scs...)
		default:
			return nil, nil, errors.New("can only spawn writes and reads")
		}
		return sc, nc, nil
	}
	return nil, nil, errors.New("asked for something we cannot do")
}

// ContractReadID references a read contract system-wide.
var ContractReadID = "scarabRead"

// ContractRead is used to create read instances that prove a reader has access
// to a given write instance. The following instructions are accepted:
//
//  - spawn:scarabRead which does some health-checks to make sure that the read
//  request is valid.
//
// TODO: correctly handle multi signatures for read requests: to whom should the
// secret be re-encrypted to? Perhaps for multi signatures we only want to have
// ephemeral keys.
func (s *Service) ContractRead(cdb ol.CollectionView, inst ol.Instruction, c []ol.Coin) ([]ol.StateChange, []ol.Coin, error) {
	if inst.Spawn == nil {
		return nil, nil, errors.New("not a spawn instruction")
	}
	if inst.Spawn.ContractID != ContractReadID {
		return nil, nil, errors.New("can only spawn read instances")
	}
	r := inst.Spawn.Args.Search("read")
	if r == nil || len(r) == 0 {
		return nil, nil, errors.New("need a read argument")
	}
	var re Read
	err := protobuf.DecodeWithConstructors(r, &re, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, nil, errors.New("passed read argument is invalid: " + err.Error())
	}
	_, cid, err := cdb.GetValues(re.Write.Slice())
	if err != nil {
		return nil, nil, errors.New("referenced write-id is not correct: " + err.Error())
	}
	if cid != ContractWriteID {
		return nil, nil, errors.New("referenced write-id is not a write instance")
	}
	re.Xc = cothority.Suite.Point()
	for _, s := range inst.Signatures {
		re.Xc.Add(re.Xc, s.Signer.Ed25519.Point)
	}
	return ol.StateChanges{ol.NewStateChange(ol.Create, inst.DeriveID("read"), ContractReadID, r)}, c, nil
}
