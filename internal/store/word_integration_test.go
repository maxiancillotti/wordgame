//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/maxiancillotti/wordgame/internal/service"
)

func TestStoreGetMaxWordID(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	maxID, err := st.GetMaxWordID(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, maxID)

	insertWord(t, "APPLE")
	w2 := insertWord(t, "BANANA")

	maxID, err = st.GetMaxWordID(ctx)
	require.NoError(t, err)
	assert.Equal(t, w2.ID, maxID)
}

func TestStoreGetWordByID(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	w := insertWord(t, "APPLE")

	got, err := st.GetWordByID(ctx, w.ID)
	require.NoError(t, err)
	assert.Equal(t, w, got)

	_, err = st.GetWordByID(ctx, w.ID+999)
	require.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrNotFound))
}
