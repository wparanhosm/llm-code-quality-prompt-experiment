package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"wallet-p2p/internal/domain"
)

func TestNewMoney(t *testing.T) {
	t.Parallel()

	t.Run("cria money com valor positivo", func(t *testing.T) {
		t.Parallel()
		m, err := domain.NewMoney(1000)
		require.NoError(t, err)
		assert.Equal(t, int64(1000), m.Cents())
	})

	t.Run("cria money com zero", func(t *testing.T) {
		t.Parallel()
		m, err := domain.NewMoney(0)
		require.NoError(t, err)
		assert.True(t, m.IsZero())
	})

	t.Run("rejeita valor negativo", func(t *testing.T) {
		t.Parallel()
		_, err := domain.NewMoney(-1)
		require.ErrorIs(t, err, domain.ErrInvalidAmount)
	})
}

func TestMoneyOperations(t *testing.T) {
	t.Parallel()

	t.Run("Add soma valores e retorna novo Money (imutável)", func(t *testing.T) {
		t.Parallel()
		a := domain.MustMoney(500)
		b := domain.MustMoney(300)
		sum := a.Add(b)
		assert.Equal(t, int64(800), sum.Cents())
		assert.Equal(t, int64(500), a.Cents(), "Add não deve mutar o receptor")
	})

	t.Run("Sub subtrai valores", func(t *testing.T) {
		t.Parallel()
		a := domain.MustMoney(500)
		result, err := a.Sub(domain.MustMoney(200))
		require.NoError(t, err)
		assert.Equal(t, int64(300), result.Cents())
	})

	t.Run("Sub até zero é permitido", func(t *testing.T) {
		t.Parallel()
		a := domain.MustMoney(500)
		result, err := a.Sub(domain.MustMoney(500))
		require.NoError(t, err)
		assert.True(t, result.IsZero())
	})

	t.Run("Sub que resultaria negativo retorna ErrInsufficientFunds", func(t *testing.T) {
		t.Parallel()
		a := domain.MustMoney(100)
		_, err := a.Sub(domain.MustMoney(101))
		require.ErrorIs(t, err, domain.ErrInsufficientFunds)
	})

	t.Run("GreaterThanOrEqual compara corretamente", func(t *testing.T) {
		t.Parallel()
		a := domain.MustMoney(100)
		assert.True(t, a.GreaterThanOrEqual(domain.MustMoney(100)))
		assert.True(t, a.GreaterThanOrEqual(domain.MustMoney(99)))
		assert.False(t, a.GreaterThanOrEqual(domain.MustMoney(101)))
	})
}
