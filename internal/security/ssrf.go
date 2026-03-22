package security

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// SSRFConfig holds SSRF protection configuration.
type SSFRConfig struct {
	// AllowPrivateIPs whether to allow private IP addresses.
	AllowPrivateIPs bool
	// AllowLoopback whether to allow loopback addresses (127.0.0.1, ::1).
	AllowLoopback bool
	// AllowCloudMetadata whether to allow cloud metadata endpoints.
	AllowCloudMetadata bool
	// BlockedHosts list of hostnames to block.
	BlockedHosts []string
	// AllowedSchemes list of allowed URL schemes.
	AllowedSchemes []string
}

// DefaultSSRFConfig returns the default SSRF protection configuration.
func DefaultSSRFConfig() *SSFRConfig {
	return &SSFRConfig{
		AllowPrivateIPs:    false,
		AllowLoopback:      false,
		AllowCloudMetadata: false,
		BlockedHosts: []string{
			"localhost",
			"127.0.0.1",
			"::1",
		},
		AllowedSchemes: []string{
			"http",
			"https",
		},
	}
}

// SSRFProtector provides protection against Server-Side Request Forgery attacks.
type SSRFProtector struct {
	config *SSFRConfig
}

// NewSSRFProtector creates a new SSRF protector with the given configuration.
func NewSSRFProtector(config *SSFRConfig) *SSRFProtector {
	if config == nil {
		config = DefaultSSRFConfig()
	}
	return &SSRFProtector{
		config: config,
	}
}

// ValidateURL validates a URL against SSRF protections.
func (p *SSRFProtector) ValidateURL(rawURL string) error {
	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Check scheme
	if !p.isSchemeAllowed(parsedURL.Scheme) {
		return fmt.Errorf("disallowed URL scheme: %s", parsedURL.Scheme)
	}

	// Extract hostname
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return errors.New("empty hostname in URL")
	}

	// Check blocked hosts
	for _, blockedHost := range p.config.BlockedHosts {
		if strings.EqualFold(hostname, blockedHost) {
			return fmt.Errorf("blocked hostname: %s", hostname)
		}
	}

	// Resolve hostname to IP addresses
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname: %w", err)
	}

	// Check each IP address
	for _, ip := range ips {
		if err := p.validateIP(ip); err != nil {
			return err
		}
	}

	return nil
}

// isSchemeAllowed checks if the URL scheme is allowed.
func (p *SSRFProtector) isSchemeAllowed(scheme string) bool {
	if len(p.config.AllowedSchemes) == 0 {
		return true
	}
	for _, allowed := range p.config.AllowedSchemes {
		if strings.EqualFold(scheme, allowed) {
			return true
		}
	}
	return false
}

// validateIP validates an IP address against SSRF protections.
func (p *SSRFProtector) validateIP(ip net.IP) error {
	// Check for loopback addresses
	if ip.IsLoopback() && !p.config.AllowLoopback {
		return fmt.Errorf("loopback address not allowed: %s", ip)
	}

	// Check for private IP addresses
	if (ip.IsPrivate() || isPrivateIPv4(ip) || isPrivateIPv6(ip)) && !p.config.AllowPrivateIPs {
		return fmt.Errorf("private IP address not allowed: %s", ip)
	}

	// Check for cloud metadata endpoints
	if isCloudMetadataEndpoint(ip) && !p.config.AllowCloudMetadata {
		return fmt.Errorf("cloud metadata endpoint not allowed: %s", ip)
	}

	return nil
}

// isPrivateIPv4 checks if an IPv4 address is in a private range.
func isPrivateIPv4(ip net.IP) bool {
	if ip.To4() == nil {
		return false
	}
	// Check RFC 1918 private ranges:
	// 10.0.0.0/8
	// 172.16.0.0/12
	// 192.168.0.0/16
	privateRanges := []*net.IPNet{
		{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(8, 32)},
		{IP: net.ParseIP("172.16.0.0"), Mask: net.CIDRMask(12, 32)},
		{IP: net.ParseIP("192.168.0.0"), Mask: net.CIDRMask(16, 32)},
	}
	for _, r := range privateRanges {
		if r.Contains(ip) {
			return true
		}
	}
	return false
}

// isPrivateIPv6 checks if an IPv6 address is in a private range.
func isPrivateIPv6(ip net.IP) bool {
	if ip.To16() == nil || ip.To4() != nil {
		return false
	}
	// Check ULA (Unique Local Address) range: fc00::/7
	ulaRange := &net.IPNet{
		IP:   net.ParseIP("fc00::"),
		Mask: net.CIDRMask(7, 128),
	}
	return ulaRange.Contains(ip)
}

// isCloudMetadataEndpoint checks if an IP address is a cloud metadata endpoint.
func isCloudMetadataEndpoint(ip net.IP) bool {
	cloudMetadataIPs := []net.IP{
		// AWS, GCP, Azure, Oracle Cloud
		net.ParseIP("169.254.169.254"),
		// Alibaba Cloud
		net.ParseIP("100.100.100.200"),
		// DigitalOcean
		net.ParseIP("169.254.169.254"),
	}
	for _, metadataIP := range cloudMetadataIPs {
		if ip.Equal(metadataIP) {
			return true
		}
	}
	return false
}

// GlobalSSRFProtector is the global SSRF protector instance.
var GlobalSSRFProtector = NewSSRFProtector(DefaultSSRFConfig())

// ValidateURL is a convenience function to validate a URL using the global protector.
func ValidateURL(rawURL string) error {
	return GlobalSSRFProtector.ValidateURL(rawURL)
}
