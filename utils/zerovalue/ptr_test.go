package zerovalue

import "testing"

func TestPtr_Int(t *testing.T) {
	p := Ptr(10)
	if p == nil || *p != 10 {
		t.Fatalf("Ptr(10) = %v, want pointer to 10", p)
	}
}

func TestPtr_String(t *testing.T) {
	p := Ptr("hello")
	if p == nil || *p != "hello" {
		t.Fatalf(`Ptr("hello") = %v, want pointer to "hello"`, p)
	}
}

func TestPtr_Bool(t *testing.T) {
	p := Ptr(true)
	if p == nil || *p != true {
		t.Fatalf("Ptr(true) = %v, want pointer to true", p)
	}
}

func TestPtr_StructByValue(t *testing.T) {
	type sample struct{ N int }
	v := sample{N: 7}
	p := Ptr(v)

	if p == nil || p.N != 7 {
		t.Fatalf("Ptr(sample) = %v, want pointer to {N:7}", p)
	}

	v.N = 99
	if p.N != 7 {
		t.Fatalf("Ptr captured by reference; want copy semantics, got p.N = %d", p.N)
	}
}

func TestPtr_DistinctAddresses(t *testing.T) {
	p1 := Ptr(1)
	p2 := Ptr(1)
	if p1 == p2 {
		t.Fatalf("Ptr returned the same address for two calls; each call must allocate")
	}
}
