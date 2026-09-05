package main

import "testing"

func TestWorldToScreen(t *testing.T) {
	c := NewCamera(160, 160)
	c.x = 40
	c.y = 20

	sx, sy := c.WorldToScreen(100, 80)
	if sx != 60 || sy != 60 {
		t.Fatalf("expected 60,60, got %v,%v", sx, sy)
	}
}

func TestFollowCenters(t *testing.T) {
	c := NewCamera(160, 160)
	c.SetWorld(320, 320)

	c.Follow(100, 100)
	if c.x != 20 || c.y != 20 {
		t.Fatalf("expected camera at 20,20, got %v,%v", c.x, c.y)
	}
}

func TestFollowClampsToWorld(t *testing.T) {
	c := NewCamera(160, 160)
	c.SetWorld(320, 320)

	c.Follow(40, 40)
	if c.x != 0 || c.y != 0 {
		t.Fatalf("expected camera clamped to 0,0, got %v,%v", c.x, c.y)
	}

	c.Follow(280, 280)
	if c.x != 160 || c.y != 160 {
		t.Fatalf("expected camera clamped to 160,160, got %v,%v", c.x, c.y)
	}
}

func TestFollowWorldSmallerThanCamera(t *testing.T) {
	c := NewCamera(200, 200)
	c.SetWorld(100, 100)

	c.Follow(50, 50)
	// worldW - width < 0, so clamping should pin to 0.
	if c.x != 0 || c.y != 0 {
		t.Fatalf("expected camera at 0,0, got %v,%v", c.x, c.y)
	}
}
