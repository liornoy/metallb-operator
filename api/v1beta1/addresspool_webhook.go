/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging addresspool-webhook.
var (
	addresspoollog    = logf.Log.WithName("addresspool-webhook")
	addressPoolClient client.Client
)

func (addressPool *AddressPool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	addressPoolClient = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(addressPool).
		Complete()
}

//+kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1beta1-addresspool,mutating=false,failurePolicy=fail,groups=metallb.io,resources=addresspools,versions=v1beta1,name=addresspoolvalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &AddressPool{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for AddressPool.
func (addressPool *AddressPool) ValidateCreate() error {
	addresspoollog.Info("validate AddressPool creation", "name", addressPool.Name)

	existingAddressPoolList, err := getExistingAddressPools()
	if err != nil {
		return err
	}

	return addressPool.validateAddressPool(true, existingAddressPoolList)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for AddressPool.
func (addressPool *AddressPool) ValidateUpdate(old runtime.Object) error {
	addresspoollog.Info("validate AddressPool update", "name", addressPool.Name)

	existingAddressPoolList, err := getExistingAddressPools()
	if err != nil {
		return err
	}

	return addressPool.validateAddressPool(false, existingAddressPoolList)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for AddressPool.
func (addressPool *AddressPool) ValidateDelete() error {
	addresspoollog.Info("validate AddressPool deletion", "name", addressPool.Name)

	return nil
}

func (addressPool *AddressPool) validateAddressPool(isNewAddressPool bool, existingAddressPoolList *AddressPoolList) error {
	addressPoolCIDRS, err := getAddressPoolCIDRs(addressPool)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse addresses for %s", addressPool.Name)
	}

	// Check protocol is BGP when BGPAdvertisement is used.
	if len(addressPool.Spec.BGPAdvertisements) != 0 {
		if addressPool.Spec.Protocol != "bgp" {
			return fmt.Errorf("bgpadvertisement config not valid for protocol %s", addressPool.Spec.Protocol)
		}
		err := validateBGPAdvertisements(addressPool.Spec.BGPAdvertisements, addressPool.Spec.Addresses)
		if err != nil {
			return errors.Wrapf(err, "invalid bgpadvertisement config")
		}
	}

	for _, existingAddressPool := range existingAddressPoolList.Items {
		if existingAddressPool.Name == addressPool.Name {
			// Check that the pool isn't already defined.
			// Avoid errors when comparing the AddressPool to itself.
			if isNewAddressPool {
				return fmt.Errorf("duplicate definition of pool %s", addressPool.Name)
			} else {
				continue
			}
		}

		existingAddressPoolCIDRS, err := getAddressPoolCIDRs(&existingAddressPool)
		if err != nil {
			return errors.Wrapf(err, "Failed to parse addresses for %s", existingAddressPool.Name)
		}

		// Check that the specified CIDR ranges are not overlapping in existing CIDRs.
		for _, existingCIDR := range existingAddressPoolCIDRS {
			for _, cidr := range addressPoolCIDRS {
				if cidrsOverlap(existingCIDR, cidr) {
					return fmt.Errorf("CIDR %q in pool %s overlaps with already defined CIDR %q in pool %s", cidr, addressPool.Name, existingCIDR, existingAddressPool.Name)
				}
			}
		}
	}
	return nil
}

func getExistingAddressPools() (*AddressPoolList, error) {
	existingAddressPoolList := &AddressPoolList{}
	err := addressPoolClient.List(context.Background(), existingAddressPoolList)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get existing addresspool objects")
	}
	return existingAddressPoolList, nil
}

func getAddressPoolCIDRs(addressPool *AddressPool) ([]*net.IPNet, error) {
	var CIDRs []*net.IPNet
	for _, cidr := range addressPool.Spec.Addresses {
		nets, err := parseCIDR(cidr)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid CIDR %q in pool %s", cidr, addressPool.Name)
		}
		CIDRs = append(CIDRs, nets...)
	}
	return CIDRs, nil
}

func validateBGPAdvertisements(bgpAdvertisements []BgpAdvertisement, poolAddresses []string) error {
	if len(bgpAdvertisements) == 0 {
		return nil
	}

	err := validateDuplicateAdvertisements(bgpAdvertisements)
	if err != nil {
		return err
	}

	for _, adv := range bgpAdvertisements {
		err := validateDuplicateCommunities(adv.Communities)
		if err != nil {
			return err
		}

		if adv.AggregationLength != nil {
			err := validateAggregationLength(*adv.AggregationLength, false, poolAddresses)
			if err != nil {
				return err
			}
		}

		if adv.AggregationLengthV6 != nil {
			err := validateAggregationLength(*adv.AggregationLengthV6, true, poolAddresses)
			if err != nil {
				return err
			}
		}

		for _, community := range adv.Communities {
			fs := strings.Split(community, ":")
			if len(fs) != 2 {
				return fmt.Errorf("invalid community string %q", community)
			}

			_, err := strconv.ParseUint(fs[0], 10, 16)
			if err != nil {
				return fmt.Errorf("invalid first section of community %q: %s", fs[0], err)
			}

			_, err = strconv.ParseUint(fs[1], 10, 16)
			if err != nil {
				return fmt.Errorf("invalid second section of community %q: %s", fs[1], err)
			}
		}
	}

	return nil
}

func validateDuplicateAdvertisements(bgpAdvertisements []BgpAdvertisement) error {
	for i := 0; i < len(bgpAdvertisements); i++ {
		for j := i + 1; j < len(bgpAdvertisements); j++ {
			if reflect.DeepEqual(bgpAdvertisements[i], bgpAdvertisements[j]) {
				return errors.New("duplicate definition of bgpadvertisement")
			}
		}
	}
	return nil
}

func validateDuplicateCommunities(communities []string) error {
	for i := 0; i < len(communities); i++ {
		for j := i + 1; j < len(communities); j++ {
			if strings.Compare(communities[i], communities[j]) == 0 {
				return errors.New("duplicate definition of communities")
			}
		}
	}
	return nil
}

func validateAggregationLength(aggregationLength int32, isV6 bool, poolAddresses []string) error {
	if isV6 {
		if aggregationLength > 128 {
			return fmt.Errorf("invalid aggregation length %d for IPv6", aggregationLength)
		}
	} else if aggregationLength > 32 {
		return fmt.Errorf("invalid aggregation length %d for IPv4", aggregationLength)
	}

	cidrsPerAddresses := map[string][]*net.IPNet{}
	for _, cidr := range poolAddresses {
		nets, err := parseCIDR(cidr)
		if err != nil {
			return errors.Wrapf(err, "failed to parse CIDR %s to validate aggregation length %d", cidr, aggregationLength)
		}
		cidrsPerAddresses[cidr] = nets
	}

	for addr, cidrs := range cidrsPerAddresses {
		if len(cidrs) == 0 {
			continue
		}

		if isV6 && cidrs[0].IP.To4() != nil {
			continue
		}

		// in case of range format, we may have a set of cidrs associated to a given address.
		// We reject if none of the cidrs are compatible with the aggregation length.
		lowest := lowestMask(cidrs)
		if aggregationLength < int32(lowest) {
			return fmt.Errorf("invalid aggregation length %d: prefix %d in "+
				"this pool is more specific than the aggregation length for addresses %s", aggregationLength, lowest, addr)
		}
	}

	return nil
}

func lowestMask(cidrs []*net.IPNet) int {
	if len(cidrs) == 0 {
		return 0
	}
	lowest, _ := cidrs[0].Mask.Size()
	for _, c := range cidrs {
		s, _ := c.Mask.Size()
		if lowest > s {
			lowest = s
		}
	}
	return lowest
}