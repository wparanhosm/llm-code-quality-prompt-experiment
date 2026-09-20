package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"wallet-p2p/internal/domain"
)

var fixedNow = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

func TestNewWallet(t *testing.T) {
	t.Parallel()

	t.Run("cria wallet com ID explícito", func(t *testing.T) {
		t.Parallel()
		w, err := domain.NewWallet("wallet-1", domain.MustMoney(1000), fixedNow)
		require.NoError(t, err)
		assert.Equal(t, "wallet-1", w.ID())
		assert.Equal(t, int64(1000), w.Balance().Cents())
		assert.Equal(t, fixedNow, w.CreatedAt())
		assert.Equal(t, fixedNow, w.UpdatedAt())
	})

	t.Run("gera UUID quando ID vazio", func(t *testing.T) {
		t.Parallel()
		w, err := domain.NewWallet("", domain.MustMoney(0), fixedNow)
		require.NoError(t, err)
		assert.NotEmpty(t, w.ID())
		assert.Len(t, w.ID(), 36, "ID gerado deve ser um UUID v4")
	})
}

func TestReconstructWallet(t *testing.T) {
	t.Parallel()

	w := domain.ReconstructWallet("w1", 750, fixedNow, fixedNow)
	assert.Equal(t, "w1", w.ID())
	assert.Equal(t, int64(750), w.Balance().Cents())
	assert.Equal(t, fixedNow, w.CreatedAt())
	assert.Equal(t, fixedNow, w.UpdatedAt())
}

func TestWalletDebit(t *testing.T) {
	t.Parallel()

	later := fixedNow.Add(time.Minute)

	t.Run("debita com saldo suficiente", func(t *testing.T) {
		t.Parallel()
		w, err := domain.NewWallet("w1", domain.MustMoney(1000), fixedNow)
		require.NoError(t, err)

		updated, err := w.Debit(domain.MustMoney(400), later)
		require.NoError(t, err)
		assert.Equal(t, int64(600), updated.Balance().Cents())
		assert.Equal(t, later, updated.UpdatedAt())
		assert.Equal(t, int64(1000), w.Balance().Cents(), "Debit deve ser imutável sobre a wallet original")
	})

	t.Run("rejeita débito com saldo insuficiente", func(t *testing.T) {
		t.Parallel()
		w, err := domain.NewWallet("w1", domain.MustMoney(100), fixedNow)
		require.NoError(t, err)

		_, err = w.Debit(domain.MustMoney(101), later)
		require.ErrorIs(t, err, domain.ErrInsufficientFunds)
	})
}

func TestWalletCanDebit(t *testing.T) {
	t.Parallel()

	w, err := domain.NewWallet("w1", domain.MustMoney(100), fixedNow)
	require.NoError(t, err)

	assert.True(t, w.CanDebit(domain.MustMoney(100)))
	assert.True(t, w.CanDebit(domain.MustMoney(50)))
	assert.False(t, w.CanDebit(domain.MustMoney(101)))
}

func TestWalletCredit(t *testing.T) {
	t.Parallel()

	later := fixedNow.Add(time.Minute)
	w, err := domain.NewWallet("w1", domain.MustMoney(100), fixedNow)
	require.NoError(t, err)

	updated := w.Credit(domain.MustMoney(50), later)
	assert.Equal(t, int64(150), updated.Balance().Cents())
	assert.Equal(t, later, updated.UpdatedAt())
	assert.Equal(t, int64(100), w.Balance().Cents(), "Credit deve ser imutável sobre a wallet original")
}
