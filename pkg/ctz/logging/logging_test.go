package logging

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestJobStatusLogger_Path(t *testing.T) {
	root := NewRootLogger("Foo")
	assert.Len(t, root.Path(), 1)
	assert.Len(t, root.PathKeys(), 1)
	assert.Equal(t, "Foo", root.PathFormatted())
	child := root.MakeOrReplaceChild("Bar", true)
	assert.Len(t, child.Path(), 2)
	assert.Len(t, child.PathKeys(), 2)
	assert.Equal(t, "Foo : Bar", child.PathFormatted())
	childNp := root.MakeOrReplaceChild("Baz", false)
	assert.Len(t, childNp.Path(), 1)
	assert.Len(t, childNp.PathKeys(), 1)
	assert.Equal(t, "Baz", childNp.PathFormatted())
}
