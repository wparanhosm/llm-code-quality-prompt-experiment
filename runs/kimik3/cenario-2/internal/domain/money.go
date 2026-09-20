// Package domain contém as entidades puras do negócio, value objects e erros
// de domínio. Esta camada NÃO depende de frameworks, bancos de dados ou
// interfaces externas (Clean Architecture + DDD).
package domain

// Money é um value object que representa um valor monetário em centavos
// (unidade mínima indivisível da moeda). Armazenar valores em inteiros
// elimina erros de arredondamento de ponto flutuante.
//
// Invariante: Money nunca pode ser negativo.
type Money struct {
	cents int64
}

// NewMoney cria um Money a partir de um valor em centavos.
// Retorna ErrInvalidAmount se o valor for negativo.
func NewMoney(cents int64) (Money, error) {
	if cents < 0 {
		return Money{}, ErrInvalidAmount
	}
	return Money{cents: cents}, nil
}

// MustMoney cria um Money sem validação. Destinado apenas a testes e a
// reconstrução de valores já validados vindos da persistência.
func MustMoney(cents int64) Money {
	return Money{cents: cents}
}

// Cents retorna o valor em centavos.
func (m Money) Cents() int64 {
	return m.cents
}

// IsZero indica se o valor é zero.
func (m Money) IsZero() bool {
	return m.cents == 0
}

// Add retorna um novo Money com a soma dos valores. Money é imutável.
func (m Money) Add(other Money) Money {
	return Money{cents: m.cents + other.cents}
}

// Sub retorna um novo Money com a diferença dos valores.
// Retorna ErrInsufficientFunds se o resultado for negativo, preservando
// a invariante de que Money nunca é negativo.
func (m Money) Sub(other Money) (Money, error) {
	result := m.cents - other.cents
	if result < 0 {
		return Money{}, ErrInsufficientFunds
	}
	return Money{cents: result}, nil
}

// GreaterThanOrEqual compara dois valores monetários.
func (m Money) GreaterThanOrEqual(other Money) bool {
	return m.cents >= other.cents
}
