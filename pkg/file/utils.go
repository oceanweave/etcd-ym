package file

import "net/url"

// 解析 backupURL
// s3://bucket-name/my-dir/object-name.db
// s3 bucket-name mydir/object-name.db
func ParseBackupURL(backupURL string) (string, string, string, error) {
	u, err := url.Parse(backupURL)
	if err != nil {
		return "", "", "", err
	}
	return u.Scheme, u.Host, u.Path[1:], nil
}
