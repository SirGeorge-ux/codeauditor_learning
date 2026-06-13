package services

import (
	"testing"
)

func TestCalculateRank_Junior(t *testing.T) {
	if r := calculateRank(0); r != RankJunior {
		t.Errorf("expected Junior, got %s", r)
	}
	if r := calculateRank(50); r != RankJunior {
		t.Errorf("expected Junior for 50 points, got %s", r)
	}
	if r := calculateRank(99); r != RankJunior {
		t.Errorf("expected Junior for 99 points, got %s", r)
	}
}

func TestCalculateRank_Mid(t *testing.T) {
	if r := calculateRank(100); r != RankMid {
		t.Errorf("expected Mid for 100 points, got %s", r)
	}
	if r := calculateRank(200); r != RankMid {
		t.Errorf("expected Mid for 200 points, got %s", r)
	}
	if r := calculateRank(299); r != RankMid {
		t.Errorf("expected Mid for 299 points, got %s", r)
	}
}

func TestCalculateRank_Senior(t *testing.T) {
	if r := calculateRank(300); r != RankSenior {
		t.Errorf("expected Senior for 300 points, got %s", r)
	}
	if r := calculateRank(500); r != RankSenior {
		t.Errorf("expected Senior for 500 points, got %s", r)
	}
	if r := calculateRank(749); r != RankSenior {
		t.Errorf("expected Senior for 749 points, got %s", r)
	}
}

func TestCalculateRank_Architect(t *testing.T) {
	if r := calculateRank(750); r != RankArchitect {
		t.Errorf("expected Architect for 750 points, got %s", r)
	}
	if r := calculateRank(1000); r != RankArchitect {
		t.Errorf("expected Architect for 1000 points, got %s", r)
	}
}

func TestRankThresholds_Ordered(t *testing.T) {
	// Must be in descending order for calculateRank to work
	for i := 1; i < len(RankThresholds); i++ {
		if RankThresholds[i-1].MinPoints <= RankThresholds[i].MinPoints {
			t.Errorf("RankThresholds not in descending order: %d <= %d",
				RankThresholds[i-1].MinPoints, RankThresholds[i].MinPoints)
		}
	}
}
