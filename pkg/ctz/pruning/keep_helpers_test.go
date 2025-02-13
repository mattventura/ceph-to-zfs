package pruning

import (
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/models"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShallowCopySnapList(t *testing.T) {

	l1 := []models.Snapshot{
		stubSnap{name: "foo"},
		stubSnap{name: "bar"},
	}
	l2 := shallowCopySnapList(l1)

	assert.Equal(t, l1, l2)

	l1[0] = stubSnap{name: "baz"}
	assert.Equal(t, "baz", l1[0].Name())
	assert.Equal(t, "foo", l2[0].Name())

}
