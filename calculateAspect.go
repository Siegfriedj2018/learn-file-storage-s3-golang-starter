package main

// GetGCD calculates the greatest common divisor using the Euclidean algorithm
func getGCD(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// IdentifyAspectRatio takes dimensions and returns a string representation
func IdentifyAspectRatio(width, height int) string {
	if width <= 0 || height <= 0 {
		return "invalid"
	}

	gcd := getGCD(width, height)
	simplifiedW := width / gcd
	simplifiedH := height / gcd

	// Check against common standards
	if simplifiedW == 16 && simplifiedH == 9 {
		return "16:9"
	}
	if simplifiedW == 9 && simplifiedH == 16 {
		return "9:16"
	}

	// For non-standard or slightly off ratios (like your 608x1080)
	// it's helpful to return the simplified raw ratio
	return "other"
}