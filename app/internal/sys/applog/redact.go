package applog

import "regexp"

// This app handles the CARE admin password, the backup passphrase and MinIO's
// root credentials, and the Settings tab round-trips backend.env through the
// same log sink. The log's whole purpose is to be sent to someone else when
// things go wrong, so anything that looks like a credential is scrubbed on the
// way in - once written, it is too late.
//
// Deliberately over-eager: a redacted line that did not need it costs nothing,
// a clinic's database password in a support email costs a great deal.

const mask = "[redacted]"

var (
	// KEY=value / KEY: value / "KEY": "value", where the name looks secret.
	// _KEY (not bare KEY) so FILE_UPLOAD_BUCKET and friends are left alone while
	// MINIO_SECRET_KEY is caught.
	assignment = regexp.MustCompile(
		`(?i)("?[A-Z0-9_]*(?:PASSWORD|PASSWD|PASSPHRASE|SECRET|TOKEN|APIKEY|_KEY)[A-Z0-9_]*"?\s*[:=]\s*)("?)([^\s"',}]+)`)

	// Credentials embedded in a URL: https://user:pass@host - git remotes and
	// registry URLs both carry these.
	urlCreds = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://[^\s:/@]+:)([^\s@/]+)(@)`)
)

// Redact masks credential-shaped values in one line.
func Redact(line string) string {
	line = assignment.ReplaceAllString(line, "${1}${2}"+mask)
	return urlCreds.ReplaceAllString(line, "${1}"+mask+"${3}")
}
