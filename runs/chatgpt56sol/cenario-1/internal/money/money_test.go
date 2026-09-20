package money

import "testing"

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{name: "whole", input: "10", want: 1000},
		{name: "one decimal", input: "10.5", want: 1050},
		{name: "two decimals", input: "0.01", want: 1},
		{name: "zero", input: "0", wantErr: true},
		{name: "negative", input: "-1.00", wantErr: true},
		{name: "too precise", input: "1.001", wantErr: true},
		{name: "comma", input: "1,00", wantErr: true},
		{name: "spaces", input: " 1.00", wantErr: true},
		{name: "overflow", input: "92233720368547758.08", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(test.input)
			if (err != nil) != test.wantErr {
				t.Fatalf("Parse(%q) error = %v, wantErr %v", test.input, err, test.wantErr)
			}
			if got != test.want {
				t.Errorf("Parse(%q) = %d, want %d", test.input, got, test.want)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	t.Parallel()
	if got := Format(1050); got != "10.50" {
		t.Fatalf("Format(1050) = %q, want %q", got, "10.50")
	}
}
