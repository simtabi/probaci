package docker

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// ImageBlockedError is returned when the image-trust policy refuses to run an
// image (strict mode). It is distinct from a tool failure so callers can report
// it accurately rather than as a check failure.
type ImageBlockedError struct {
	Ref    string
	Reason string
}

func (e *ImageBlockedError) Error() string {
	return fmt.Sprintf("image %s blocked by trust policy: %s", e.Ref, e.Reason)
}

// pinned reports whether ref is pinned by digest (image@sha256:…), which makes
// it tamper-evident.
func pinned(ref string) bool { return strings.Contains(ref, "@sha256:") }

// allowListed reports whether ref's image (host/repo, ignoring tag/digest) is in
// the configured allow-list.
func (b *Broker) allowListed(ref string) bool {
	img := ref
	if i := strings.IndexAny(img, "@:"); i > 0 {
		// keep host:port intact: only strip a tag/digest after the last "/"
		if slash := strings.LastIndex(ref, "/"); slash >= 0 {
			tail := ref[slash:]
			if j := strings.IndexAny(tail, "@:"); j >= 0 {
				img = ref[:slash] + tail[:j]
			}
		} else if j := strings.IndexAny(ref, "@:"); j > 0 {
			img = ref[:j]
		}
	}
	for _, a := range b.sec.AllowUnsigned {
		if a == img || strings.HasPrefix(ref, a) {
			return true
		}
	}
	return false
}

// CheckImage enforces the image-trust policy for ref. In strict mode it returns
// an error for images that are neither digest-pinned nor allow-listed (and,
// when a cosign identity is configured, fails verification). In advisory mode it
// never blocks; callers surface unpinned images via `doctor`. The returned
// string is a non-fatal advisory note ("" when none).
func (b *Broker) CheckImage(ctx context.Context, ref string) (note string, err error) {
	strict := b.sec.VerifyImages == "strict"
	if pinned(ref) {
		if b.sec.CosignIdentity != "" {
			if vErr := b.cosignVerify(ctx, ref); vErr != nil {
				if strict {
					return "", &ImageBlockedError{Ref: ref, Reason: "cosign verification failed: " + vErr.Error()}
				}
				return "unverified (cosign): " + ref, nil
			}
		}
		return "", nil
	}
	// Unpinned.
	if b.allowListed(ref) {
		return "", nil
	}
	if strict {
		return "", &ImageBlockedError{Ref: ref, Reason: "not digest-pinned and not allow-listed; pin it or add to security.allow_unsigned"}
	}
	return "unpinned image (advisory): " + ref, nil
}

// cosignVerify runs keyless cosign verification when the binary is available.
// A missing cosign binary is treated as "cannot verify" (caller decides).
func (b *Broker) cosignVerify(ctx context.Context, ref string) error {
	if _, err := exec.LookPath("cosign"); err != nil {
		return fmt.Errorf("cosign not installed")
	}
	args := []string{"verify", "--certificate-identity", b.sec.CosignIdentity}
	if b.sec.CosignIssuer != "" {
		args = append(args, "--certificate-oidc-issuer", b.sec.CosignIssuer)
	}
	args = append(args, ref)
	return exec.CommandContext(ctx, "cosign", args...).Run()
}
