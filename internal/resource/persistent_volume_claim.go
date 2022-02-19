package resource

import "sigs.k8s.io/controller-runtime/pkg/client"

type PersistentVolumeClaimBuilder struct {
	BaseBuilder
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) PersistentVolumeClaim(profile OSRMProfile) *PersistentVolumeClaimBuilder {
	return &PersistentVolumeClaimBuilder{
		BaseBuilder{profile},
		builder,
	}
}

func (builder *PersistentVolumeClaimBuilder) Build() (client.Object, error) {
	return nil, nil
}

func (builder *PersistentVolumeClaimBuilder) Update(object client.Object) error {
	return nil
}
