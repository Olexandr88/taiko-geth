package beacon

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/internal/testlog"
	"github.com/ethereum/go-ethereum/log"
	"github.com/protolambda/zrnt/eth2/beacon/capella"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/stretchr/testify/assert"
)

var _ ConsensusAPI = (*MockConsensusAPI)(nil)

type MockConsensusAPI struct {
	testdataDir string
}

func NewMockConsensusAPI(path string) (ConsensusAPI, error) {
	return &MockConsensusAPI{testdataDir: path}, nil
}

func (m MockConsensusAPI) GetUpdates(_, _ uint64) ([]*capella.LightClientUpdate, error) {
	jsonStr, _ := os.ReadFile(m.testdataDir + "/updates.json")

	updates := make([]*capella.LightClientUpdate, 0)
	_ = json.Unmarshal(jsonStr, &updates)

	return updates, nil
}

func (m MockConsensusAPI) GetCheckpointData(_ common.Root) (*capella.LightClientBootstrap, error) {
	jsonStr, _ := os.ReadFile(m.testdataDir + "/bootstrap.json")

	bootstrap := &capella.LightClientBootstrap{}
	_ = json.Unmarshal(jsonStr, &bootstrap)

	return bootstrap, nil
}

func (m MockConsensusAPI) GetFinalityData() (*capella.LightClientFinalityUpdate, error) {
	jsonStr, _ := os.ReadFile(m.testdataDir + "/finality.json")

	finality := &capella.LightClientFinalityUpdate{}
	_ = json.Unmarshal(jsonStr, &finality)

	return finality, nil
}

func (m MockConsensusAPI) GetOptimisticData() (*capella.LightClientOptimisticUpdate, error) {
	jsonStr, _ := os.ReadFile(m.testdataDir + "/optimistic.json")

	optimistic := &capella.LightClientOptimisticUpdate{}
	_ = json.Unmarshal(jsonStr, &optimistic)

	return optimistic, nil
}

func (m MockConsensusAPI) ChainID() uint64 {
	panic("implement me")
}

func (m MockConsensusAPI) Name() string {
	return "mock"
}

func getClient(strictCheckpointAge bool, t *testing.T) (*ConsensusLightClient, error) {
	baseConfig := Mainnet()
	api, err := NewMockConsensusAPI("testdata/mockdata")
	assert.NoError(t, err)

	config := &Config{
		ConsensusAPI:        api.Name(),
		Chain:               baseConfig.Chain,
		Spec:                baseConfig.Spec,
		StrictCheckpointAge: strictCheckpointAge,
	}

	checkpoint := common.Root(hexutil.MustDecode("0xc62aa0de55e6f21230fa63713715e1a6c13e73005e89f6389da271955d819bde"))

	client, err := NewConsensusLightClient(api, config, checkpoint, testlog.Logger(t, log.LvlTrace))
	if err != nil {
		return nil, err
	}

	return client, nil
}

func TestVerifyCheckpointAgeInvalid(t *testing.T) {
	_, err := getClient(true, t)
	assert.ErrorContains(t, err, "checkpoint is too old")
}