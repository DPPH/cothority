package scarab

import (
	"testing"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/omniledger/darc"
	ol "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/protobuf"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestService_CreateLTS(t *testing.T) {
	for _, nodes := range []int{3, 7, 10} {
		func(nodes int) {
			s := newTS(t, nodes)
			require.NotNil(t, s.ltsReply.LTSID)
			require.NotNil(t, s.ltsReply.X)
			defer s.Close()
		}(nodes)
	}
}

func TestContract_Write(t *testing.T) {
	s := newTS(t, 5)
	defer s.Close()
	s.CreateGenesis(t)
	pr := s.AddWriteAndWait(t, []byte("secret key"))
	require.Nil(t, pr.Verify(s.gbReply.Skipblock.Hash))
}

func TestContract_Write_Benchmark(t *testing.T) {
	s := newTS(t, 5)
	defer s.Close()
	s.CreateGenesis(t)
	totalTrans := 10
	var times []time.Duration

	for i := 0; i < 50; i++ {
		var iids []ol.InstanceID
		start := time.Now()
		for i := 0; i < totalTrans; i++ {
			iids = append(iids, s.AddWrite(t, []byte("secret key")))
		}
		timeSend := time.Now().Sub(start)
		log.Lvlf1("Time to send %d writes to OmniLedger: %s", totalTrans, timeSend)
		start = time.Now()
		for i := 0; i < totalTrans; i++ {
			for {
				pr, err := s.cl.WaitProof(iids[i], s.genesisMsg.BlockInterval, nil)
				if err == nil {
					require.Nil(t, pr.Verify(s.gbReply.Skipblock.Hash))
					break
				}
			}
		}
		timeWait := time.Now().Sub(start)
		log.Lvlf1("Time to wait for %d writes in OmniLedger: %s", totalTrans, timeWait)
		times = append(times, timeSend+timeWait)
		for _, ti := range times {
			log.Lvlf1("Total time: %s - tps: %f", ti,
				float64(totalTrans)/ti.Seconds())
		}
	}
}

func TestContract_Read(t *testing.T) {
	s := newTS(t, 5)
	defer s.Close()
	prWrite := s.AddWriteAndWait(t, []byte("secret key"))

	s.AddRead(t, prWrite, &Read{})
	pr := s.AddRead(t, prWrite, nil)
	require.Nil(t, pr.Verify(s.gbReply.Skipblock.Hash))
}

func TestService_DecryptKey(t *testing.T) {
	s := newTS(t, 5)
	defer s.Close()
	s.CreateGenesis(t)

	key1 := []byte("secret key 1")
	prWr1 := s.AddWriteAndWait(t, key1)
	prRe1 := s.AddRead(t, prWr1, nil)
	key2 := []byte("secret key 2")
	prWr2 := s.AddWriteAndWait(t, key2)
	prRe2 := s.AddRead(t, prWr2, nil)

	_, err := s.services[0].DecryptKey(&DecryptKey{Read: *prRe1, Write: *prWr2})
	require.NotNil(t, err)
	_, err = s.services[0].DecryptKey(&DecryptKey{Read: *prRe2, Write: *prWr1})
	require.NotNil(t, err)

	dk1, err := s.services[0].DecryptKey(&DecryptKey{Read: *prRe1, Write: *prWr1})
	require.Nil(t, err)
	keyCopy1, err := DecodeKey(cothority.Suite, s.ltsReply.X, dk1.Cs, dk1.XhatEnc, s.signer.Ed25519.Secret)
	require.Nil(t, err)
	require.Equal(t, key1, keyCopy1)

	dk2, err := s.services[0].DecryptKey(&DecryptKey{Read: *prRe2, Write: *prWr2})
	require.Nil(t, err)
	keyCopy2, err := DecodeKey(cothority.Suite, s.ltsReply.X, dk2.Cs, dk2.XhatEnc, s.signer.Ed25519.Secret)
	require.Nil(t, err)
	require.Equal(t, key2, keyCopy2)
}

type ts struct {
	local      *onet.LocalTest
	servers    []*onet.Server
	services   []*Service
	roster     *onet.Roster
	ltsReply   *CreateLTSReply
	signer     darc.Signer
	cl         *ol.Client
	gbReply    *ol.CreateGenesisBlockResponse
	genesisMsg *ol.CreateGenesisBlock
	gDarc      *darc.Darc
}

func (s *ts) AddRead(t *testing.T, write *ol.Proof, read *Read) *ol.Proof {
	var readBuf []byte
	if read == nil {
		read = &Read{
			Write: ol.NewInstanceID(write.InclusionProof.Key),
			Xc:    s.signer.Ed25519.Point,
		}
	}
	var err error
	readBuf, err = protobuf.Encode(read)
	require.Nil(t, err)
	ctx := ol.ClientTransaction{
		Instructions: ol.Instructions{{
			InstanceID: ol.NewInstanceID(write.InclusionProof.Key),
			Nonce:      ol.Nonce{},
			Index:      0,
			Length:     1,
			Spawn: &ol.Spawn{
				ContractID: ContractReadID,
				Args:       ol.Arguments{{Name: "read", Value: readBuf}},
			},
		}},
	}
	require.Nil(t, ctx.Instructions[0].SignBy(s.signer))
	_, err = s.cl.AddTransaction(ctx)
	require.Nil(t, err)
	instID := ctx.Instructions[0].DeriveID("read")
	pr, err := s.cl.WaitProof(instID, s.genesisMsg.BlockInterval, nil)
	if read != nil {
		require.Nil(t, err)
	} else {
		require.NotNil(t, err)
	}
	return pr
}

func newTS(t *testing.T, nodes int) ts {
	s := ts{}
	s.local = onet.NewLocalTestT(cothority.Suite, t)

	s.servers, s.roster, _ = s.local.GenTree(nodes, true)
	services := s.local.GetServices(s.servers, scarabID)
	for _, ser := range services {
		s.services = append(s.services, ser.(*Service))
	}
	log.Lvl2("Starting dkg for", nodes, "nodes")
	var err error
	s.ltsReply, err = s.services[0].CreateLTS(&CreateLTS{Roster: *s.roster})
	require.Nil(t, err)
	log.Lvl2("Done setting up dkg")
	s.signer = darc.NewSignerEd25519(nil, nil)
	return s
}

func (s *ts) Close() {
	for _, service := range s.local.GetServices(s.servers, ol.OmniledgerID) {
		service.(*ol.Service).ClosePolling()
	}
	s.local.WaitDone(time.Second)
	s.local.CloseAll()
}

func (s *ts) CreateGenesis(t *testing.T) {
	s.cl = ol.NewClient()

	var err error
	s.genesisMsg, err = ol.DefaultGenesisMsg(ol.CurrentVersion, s.roster,
		[]string{"spawn:scarabWrite", "spawn:scarabRead"}, s.signer.Identity())
	require.Nil(t, err)
	s.gDarc = &s.genesisMsg.GenesisDarc

	s.genesisMsg.BlockInterval = time.Second / 2

	s.gbReply, err = s.cl.CreateGenesisBlock(s.genesisMsg)
	require.Nil(t, err)

}

func (s *ts) AddWriteAndWait(t *testing.T, key []byte) *ol.Proof {
	instID := s.AddWrite(t, key)
	pr, err := s.cl.WaitProof(instID, s.genesisMsg.BlockInterval, nil)
	require.Nil(t, err)
	return pr
}

func (s *ts) AddWrite(t *testing.T, key []byte) ol.InstanceID {
	write := NewWrite(cothority.Suite, s.ltsReply.LTSID, s.gDarc.GetBaseID(), s.ltsReply.X, key)
	writeBuf, err := protobuf.Encode(write)
	require.Nil(t, err)

	ctx := ol.ClientTransaction{
		Instructions: ol.Instructions{{
			InstanceID: ol.InstanceID{DarcID: s.gDarc.GetBaseID(), SubID: ol.SubID{}},
			Nonce:      ol.Nonce{},
			Index:      0,
			Length:     1,
			Spawn: &ol.Spawn{
				ContractID: ContractWriteID,
				Args:       ol.Arguments{{Name: "write", Value: writeBuf}},
			},
		}},
	}
	require.Nil(t, ctx.Instructions[0].SignBy(s.signer))
	_, err = s.cl.AddTransaction(ctx)
	require.Nil(t, err)
	return ctx.Instructions[0].DeriveID("write")
}
