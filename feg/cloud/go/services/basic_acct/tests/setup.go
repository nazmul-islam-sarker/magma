/*
 * Copyright 2020 The Magma Authors.
 *
 * This source code is licensed under the BSD-style license found in the
 * LICENSE file in the root directory of this source tree.
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package tests

import (
	"context"
	"sync"
	"testing"

	"magma/feg/cloud/go/feg"
	"magma/feg/cloud/go/serdes"
	models2 "magma/feg/cloud/go/services/feg/obsidian/models"
	"magma/lte/cloud/go/lte"
	models3 "magma/lte/cloud/go/services/lte/obsidian/models"
	"magma/orc8r/cloud/go/orc8r"
	"magma/orc8r/cloud/go/services/configurator"
	configuratorTestInit "magma/orc8r/cloud/go/services/configurator/test_init"
	"magma/orc8r/cloud/go/services/device"
	deviceTestInit "magma/orc8r/cloud/go/services/device/test_init"
	"magma/orc8r/cloud/go/services/orchestrator/obsidian/models"
	"magma/orc8r/cloud/go/storage"

	"github.com/go-openapi/swag"
	"github.com/stretchr/testify/assert"
)

var (
	NhNetworkID           = "nh"
	ServingFegNetworkID   = "serving_feg"
	FederatedLteNetworkID = "federated_lte"
	NhImsi                = "123456000000101"
	NhPlmnId              = NhImsi[:6]
	AgwHwId               = "lte_gw_hw_id"
	AgwId                 = "lte_gw_id"
	FegHwId               = "feg_hw_id"
	FegId                 = "feg_id"

	once sync.Once
)

func SetupNetworks(t *testing.T) {
	once.Do(func() {
		configuratorTestInit.StartTestService(t)
		deviceTestInit.StartTestService(t)
	})

	nhFegCfg := models2.NewDefaultNetworkFederationConfigs()
	nhFegCfg.NhRoutes = models2.NhRoutes{NhPlmnId: ServingFegNetworkID}
	nhFegCfg.ServedNetworkIds = models2.ServedNetworkIds{FederatedLteNetworkID}

	servingFegCfg := models2.NewDefaultNetworkFederationConfigs()
	servingFegCfg.ServedNhIds = models2.ServedNhIds{NhNetworkID}

	lteNetCfg := models2.NewDefaultFederatedNetworkConfigs()
	lteNetCfg.FegNetworkID = &NhNetworkID

	// Neutral Host Network
	nhNetworkConfig := configurator.Network{
		ID:          NhNetworkID,
		Type:        feg.FederationNetworkType,
		Name:        "TestNeutralHost",
		Description: "Test Neutral Host",
		Configs: map[string]interface{}{
			feg.FegNetworkType:          nhFegCfg,
			orc8r.NetworkFeaturesConfig: models.NewDefaultFeaturesConfig(),
			orc8r.DnsdNetworkType:       models.NewDefaultDNSConfig(),
		},
	}
	// Serving FeG Network
	servingFegNetworkCfg := configurator.Network{
		ID:          ServingFegNetworkID,
		Type:        feg.FederationNetworkType,
		Name:        "serving_feg_network",
		Description: "Serving FeG Network",
		Configs: map[string]interface{}{
			feg.FegNetworkType:          servingFegCfg,
			orc8r.NetworkFeaturesConfig: models.NewDefaultFeaturesConfig(),
			orc8r.DnsdNetworkType:       models.NewDefaultDNSConfig(),
		},
	}
	// Federated LTE Network
	federatedLteNetCfg := configurator.Network{
		ID:          FederatedLteNetworkID,
		Type:        feg.FederatedLteNetworkType,
		Name:        "Federated_FeG_Network",
		Description: "Federated FeG Network",
		Configs: map[string]interface{}{
			feg.FederatedNetworkType:      lteNetCfg,
			lte.CellularNetworkConfigType: models3.NewDefaultTDDNetworkConfig(),
			orc8r.NetworkFeaturesConfig:   models.NewDefaultFeaturesConfig(),
			orc8r.DnsdNetworkType:         models.NewDefaultDNSConfig(),
		},
	}
	networkConfigs := []configurator.Network{
		nhNetworkConfig,
		servingFegNetworkCfg,
		federatedLteNetCfg,
	}
	_, err := configurator.CreateNetworks(context.Background(), networkConfigs, serdes.Network)
	assert.NoError(t, err)

	_, err = configurator.CreateEntities(
		FederatedLteNetworkID,
		[]configurator.NetworkEntity{
			{Type: lte.CellularEnodebEntityType, Key: "enb1"},
			{Type: lte.CellularEnodebEntityType, Key: "enb2"},
			{
				Type: lte.CellularGatewayEntityType, Key: AgwId,
				Config: &models3.GatewayCellularConfigs{
					Epc: &models3.GatewayEpcConfigs{NatEnabled: swag.Bool(true), IPBlock: "192.168.0.0/24"},
					Ran: &models3.GatewayRanConfigs{Pci: 260, TransmitEnabled: swag.Bool(true)},
				},
				Associations: []storage.TypeAndKey{
					{Type: lte.CellularEnodebEntityType, Key: "enb1"},
					{Type: lte.CellularEnodebEntityType, Key: "enb2"},
				},
			},
			{
				Type: orc8r.MagmadGatewayType, Key: AgwId,
				Name: "lte_gateway", Description: "federated lte gateway",
				PhysicalID: AgwHwId,
				Config: &models.MagmadGatewayConfigs{
					AutoupgradeEnabled:      swag.Bool(true),
					AutoupgradePollInterval: 300,
					CheckinInterval:         15,
					CheckinTimeout:          5,
				},
				Associations: []storage.TypeAndKey{{Type: lte.CellularGatewayEntityType, Key: AgwId}},
			},
			{
				Type: orc8r.UpgradeTierEntityType, Key: "t1",
				Associations: []storage.TypeAndKey{
					{Type: orc8r.MagmadGatewayType, Key: AgwId},
				},
			},
		},
		serdes.Entity,
	)
	assert.NoError(t, err)
	err = device.RegisterDevice(context.Background(), FederatedLteNetworkID, orc8r.AccessGatewayRecordType, AgwHwId, &models.GatewayDevice{HardwareID: AgwHwId, Key: &models.ChallengeKey{KeyType: "ECHO"}}, serdes.Device)
	assert.NoError(t, err)

	_, err = configurator.CreateEntities(
		ServingFegNetworkID,
		[]configurator.NetworkEntity{
			{
				Type: feg.FegGatewayType, Key: FegId,
			},
			{
				Type: orc8r.MagmadGatewayType, Key: FegId,
				Name: "feg_gateway", Description: "federation gateway",
				PhysicalID: FegHwId,
				Config: &models.MagmadGatewayConfigs{
					AutoupgradeEnabled:      swag.Bool(true),
					AutoupgradePollInterval: 300,
					CheckinInterval:         15,
					CheckinTimeout:          5,
				},
				Associations: []storage.TypeAndKey{{Type: feg.FegGatewayType, Key: FegId}},
			},
			{
				Type: orc8r.UpgradeTierEntityType, Key: "t1",
				Associations: []storage.TypeAndKey{
					{Type: orc8r.MagmadGatewayType, Key: FegId},
				},
			},
		},
		serdes.Entity,
	)
	assert.NoError(t, err)
	err = device.RegisterDevice(context.Background(), ServingFegNetworkID, orc8r.AccessGatewayRecordType, FegHwId, &models.GatewayDevice{HardwareID: FegHwId, Key: &models.ChallengeKey{KeyType: "ECHO"}}, serdes.Device)
	assert.NoError(t, err)

	actualNHNet, err := configurator.LoadNetwork(context.Background(), NhNetworkID, true, true, serdes.Network)
	assert.NoError(t, err)
	assert.Equal(t, nhNetworkConfig, actualNHNet)

	actualFeGNet, err := configurator.LoadNetwork(context.Background(), ServingFegNetworkID, true, true, serdes.Network)
	assert.NoError(t, err)
	assert.Equal(t, servingFegNetworkCfg, actualFeGNet)

}
