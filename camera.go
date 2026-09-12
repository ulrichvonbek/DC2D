package main

type Camera struct {
	width, height  float64 //viewport size
	worldW, worldH float64 //dungeon size, can vary between levels
	x, y           float64 //top left corner of the visible area
}

func NewCamera(width, height int) *Camera {
	return &Camera{
		width:  float64(width),
		height: float64(height),
	}
}

func (c *Camera) SetWorld(width, height int) {
	c.worldW = float64(width)
	c.worldH = float64(height)
}

func (c *Camera) Follow(targetX, targetY float64) {
	c.x = targetX - c.width/2
	c.y = targetY - c.height/2

	if c.worldW <= c.width {
		c.x = 0
	} else if c.x < 0 {
		c.x = 0
	} else if c.x > c.worldW-c.width {
		c.x = c.worldW - c.width
	}

	if c.worldH <= c.height {
		c.y = 0
	} else if c.y < 0 {
		c.y = 0
	} else if c.y > c.worldH-c.height {
		c.y = c.worldH - c.height
	}
}

func (c *Camera) ViewX() int {
	return int(c.x)
}

func (c *Camera) ViewY() int {
	return int(c.y)
}

func (c *Camera) WorldToScreen(wx, wy float64) (float64, float64) {
	return wx - c.x, wy - c.y
}
