package xtransaction

import "math"

// cubic implements the cubic-bezier easing solver x.com uses to derive the
// animation key. Ported from twikit's Cubic class.
type cubic struct {
	curves []float64
}

func newCubic(curves []float64) *cubic { return &cubic{curves: curves} }

func (c *cubic) getValue(t float64) float64 {
	var startGradient, endGradient, start, mid float64
	end := 1.0

	if t <= 0.0 {
		if c.curves[0] > 0.0 {
			startGradient = c.curves[1] / c.curves[0]
		} else if c.curves[1] == 0.0 && c.curves[2] > 0.0 {
			startGradient = c.curves[3] / c.curves[2]
		}
		return startGradient * t
	}

	if t >= 1.0 {
		if c.curves[2] < 1.0 {
			endGradient = (c.curves[3] - 1.0) / (c.curves[2] - 1.0)
		} else if c.curves[2] == 1.0 && c.curves[0] < 1.0 {
			endGradient = (c.curves[1] - 1.0) / (c.curves[0] - 1.0)
		}
		return 1.0 + endGradient*(t-1.0)
	}

	for start < end {
		mid = (start + end) / 2
		xEst := cubicCalc(c.curves[0], c.curves[2], mid)
		if math.Abs(t-xEst) < 0.00001 {
			return cubicCalc(c.curves[1], c.curves[3], mid)
		}
		if xEst < t {
			start = mid
		} else {
			end = mid
		}
	}
	return cubicCalc(c.curves[1], c.curves[3], mid)
}

func cubicCalc(a, b, m float64) float64 {
	return 3.0*a*(1-m)*(1-m)*m + 3.0*b*(1-m)*m*m + m*m*m
}

// interpolate linearly interpolates each element of from→to by factor f.
func interpolate(from, to []float64, f float64) []float64 {
	out := make([]float64, len(from))
	for i := range from {
		out[i] = from[i]*(1-f) + to[i]*f
	}
	return out
}

// convertRotationToMatrix returns the 2x2 rotation matrix for a rotation in
// degrees, flattened to [cos, -sin, sin, cos].
func convertRotationToMatrix(rotation float64) []float64 {
	rad := rotation * math.Pi / 180
	return []float64{math.Cos(rad), -math.Sin(rad), math.Sin(rad), math.Cos(rad)}
}

// isOdd mirrors twikit's is_odd: -1.0 for odd, 0.0 for even.
func isOdd(n int) float64 {
	if n%2 != 0 {
		return -1.0
	}
	return 0.0
}

// solve maps a byte value into [minVal,maxVal], optionally floored.
func solve(value, minVal, maxVal float64, rounding bool) float64 {
	result := value*(maxVal-minVal)/255 + minVal
	if rounding {
		return math.Floor(result)
	}
	return math.Round(result*100) / 100
}

// floatToHex replicates twikit's float_to_hex for the fractional-hex encoding
// used in the animation key.
func floatToHex(x float64) string {
	var result []byte
	quotient := int(x)
	fraction := x - float64(quotient)

	for quotient > 0 {
		quotient = int(x / 16)
		remainder := int(x - float64(quotient)*16)
		if remainder > 9 {
			result = append([]byte{byte(remainder + 55)}, result...)
		} else {
			result = append([]byte{byte('0' + remainder)}, result...)
		}
		x = float64(quotient)
	}

	if fraction == 0 {
		return string(result)
	}
	result = append(result, '.')

	for fraction > 0 {
		fraction *= 16
		integer := int(fraction)
		fraction -= float64(integer)
		if integer > 9 {
			result = append(result, byte(integer+55))
		} else {
			result = append(result, byte('0'+integer))
		}
	}
	return string(result)
}
