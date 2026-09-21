package main

import (
	"encoding/json"
	"testing"
)

func TestADR_3_SpikeRowsCountAndSize(t *testing.T) {
	rows := spikeRows(5000)
	if len(rows) != 5000 {
		t.Fatalf("len(spikeRows(5000)) = %d, want 5000", len(rows))
	}

	b, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if perRow := len(b) / len(rows); perRow < 150 || perRow > 250 {
		t.Errorf("avg marshaled row size = %d bytes, want ~180-220 (Design)", perRow)
	}
}

func TestADR_3_SpikeRowsDeterministic(t *testing.T) {
	a, err := json.Marshal(spikeRows(50))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(spikeRows(50))
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Error("spikeRows(50) is not deterministic across calls")
	}
}
