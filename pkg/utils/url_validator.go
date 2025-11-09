package utils

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateURL checks if a URL is safe to fetch, protecting against SSRF attacks.
// Returns an error if the URL is invalid or potentially dangerous.
//
// Security checks:
//   - Must be HTTP or HTTPS
//   - Must not target private/internal IP ranges (RFC1918, loopback, link-local)
//   - Must not target cloud metadata endpoints
//   - Must have a valid hostname
func ValidateURL(rawURL string) error {
	// Parse URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Check scheme
	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("invalid URL scheme: %s (only http/https allowed)", parsedURL.Scheme)
	}

	// Get hostname
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL missing hostname")
	}

	// Check for obviously dangerous hostnames
	if err := checkDangerousHostname(hostname); err != nil {
		return err
	}

	// Resolve hostname to IP addresses
	ips, err := net.LookupIP(hostname)
	if err != nil {
		// DNS resolution failed - could be legitimate (network issue) or malicious
		// For security, we reject it
		return fmt.Errorf("failed to resolve hostname %s: %w", hostname, err)
	}

	if len(ips) == 0 {
		return fmt.Errorf("no IP addresses found for hostname %s", hostname)
	}

	// Check each resolved IP
	for _, ip := range ips {
		if err := checkDangerousIP(ip); err != nil {
			return fmt.Errorf("hostname %s resolves to dangerous IP %s: %w", hostname, ip, err)
		}
	}

	return nil
}

// checkDangerousHostname checks for obviously dangerous hostnames
func checkDangerousHostname(hostname string) error {
	hostname = strings.ToLower(hostname)

	// Block localhost variants
	if hostname == "localhost" || hostname == "localhost.localdomain" {
		return fmt.Errorf("localhost not allowed")
	}

	// Block cloud metadata endpoints (common SSRF targets)
	dangerousHosts := []string{
		"169.254.169.254", // AWS, Azure, GCP metadata
		"metadata.google.internal",
		"metadata",
	}

	for _, dangerous := range dangerousHosts {
		if hostname == dangerous {
			return fmt.Errorf("blocked dangerous hostname: %s", hostname)
		}
	}

	return nil
}

// checkDangerousIP checks if an IP address is in a dangerous range
func checkDangerousIP(ip net.IP) error {
	// Loopback (127.0.0.0/8, ::1/128)
	if ip.IsLoopback() {
		return fmt.Errorf("loopback IP not allowed")
	}

	// Private networks (RFC1918: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
	if ip.IsPrivate() {
		return fmt.Errorf("private IP not allowed")
	}

	// Link-local (169.254.0.0/16, fe80::/10)
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("link-local IP not allowed")
	}

	// Cloud metadata IP (169.254.169.254) - double check
	if ip.String() == "169.254.169.254" {
		return fmt.Errorf("cloud metadata IP blocked")
	}

	// Multicast
	if ip.IsMulticast() {
		return fmt.Errorf("multicast IP not allowed")
	}

	// Unspecified (0.0.0.0, ::)
	if ip.IsUnspecified() {
		return fmt.Errorf("unspecified IP not allowed")
	}

	// Check for IPv4-mapped IPv6 addresses that might bypass checks
	if ip.To4() == nil && len(ip) == net.IPv6len {
		// Check if it's an IPv4-mapped IPv6 address (::ffff:x.x.x.x)
		if ip[10] == 0xff && ip[11] == 0xff {
			ipv4 := net.IPv4(ip[12], ip[13], ip[14], ip[15])
			return checkDangerousIP(ipv4)
		}
	}

	return nil
}
