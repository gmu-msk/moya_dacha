package main

import "testing"

func TestMaskPhone(t *testing.T) {
	for phone, want := range map[string]string{
		"+79001234567": "+7 *** ***-45-67",
		"+7900":        "+7 *** ***-**-**",
	} {
		if got := maskPhone(phone); got != want {
			t.Errorf("maskPhone(%q) = %q, ожидалось %q", phone, got, want)
		}
	}
}
