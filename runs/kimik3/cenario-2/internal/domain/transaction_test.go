package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"wallet-p2p/internal/domain"
)

func TestNewTransaction(t *testing.T) {
	t.Parallel()

	t.Run("cria transação válida", func(t *testing.T) {
		t.Parallel()
		tx, err := domain.NewTransaction("from-1", "to-1", domain.MustMoney(500), "idem-1", fixedNow)
		require.NoError(t, err)
		assert.NotEmpty(t, tx.ID())
		assert.Equal(t, "from-1", tx.FromWalletID())
		assert.Equal(t, "to-1", tx.ToWalletID())
		assert.Equal(t, int64(500), tx.Amount().Cents())
		assert.Equal(t, domain.TransactionStatusCompleted, tx.Status())
		assert.Equal(t, "idem-1", tx.IdempotencyKey())
		assert.Equal(t, fixedNow, tx.CreatedAt())
	})

	t.Run("rejeita origem vazia", func(t *testing.T) {
		t.Parallel()
		_, err := domain.NewTransaction("", "to-1", domain.MustMoney(500), "", fixedNow)
		require.ErrorIs(t, err, domain.ErrInvalidWalletID)
	})

	t.Run("rejeita destino vazio", func(t *testing.T) {
		t.Parallel()
		_, err := domain.NewTransaction("from-1", "", domain.MustMoney(500), "", fixedNow)
		require.ErrorIs(t, err, domain.ErrInvalidWalletID)
	})

	t.Run("rejeita origem igual ao destino", func(t *testing.T) {
		t.Parallel()
		_, err := domain.NewTransaction("same", "same", domain.MustMoney(500), "", fixedNow)
		require.ErrorIs(t, err, domain.ErrSameWallet)
	})

	t.Run("rejeita valor zero", func(t *testing.T) {
		t.Parallel()
		_, err := domain.NewTransaction("from-1", "to-1", domain.MustMoney(0), "", fixedNow)
		require.ErrorIs(t, err, domain.ErrInvalidAmount)
	})
}

func TestReconstructTransaction(t *testing.T) {
	t.Parallel()

	tx := domain.ReconstructTransaction("tx-1", "from-1", "to-1", 250, domain.TransactionStatusCompleted, "idem-x", fixedNow)
	assert.Equal(t, "tx-1", tx.ID())
	assert.Equal(t, "from-1", tx.FromWalletID())
	assert.Equal(t, "to-1", tx.ToWalletID())
	assert.Equal(t, int64(250), tx.Amount().Cents())
	assert.Equal(t, domain.TransactionStatusCompleted, tx.Status())
	assert.Equal(t, "idem-x", tx.IdempotencyKey())
	assert.Equal(t, fixedNow, tx.CreatedAt())
}
