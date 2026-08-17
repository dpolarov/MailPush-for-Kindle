package updater

import (
	"archive/zip"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultLatestReleaseURL = "https://api.github.com/repos/dpolarov/MailPush-for-Kindle/releases/latest"
	DefaultAssetName        = "mailpush.koplugin-update.tar"
	DefaultMaxArchiveBytes  = int64(32 * 1024 * 1024)
)

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
	Size               int64  `json:"size"`
}

type Release struct {
	TagName    string  `json:"tag_name"`
	HTMLURL    string  `json:"html_url"`
	Prerelease bool    `json:"prerelease"`
	Draft      bool    `json:"draft"`
	Assets     []Asset `json:"assets"`
}

type CheckResult struct {
	Available      bool   `json:"available"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	ReleaseURL     string `json:"release_url,omitempty"`
	AssetURL       string `json:"asset_url,omitempty"`
	AssetName      string `json:"asset_name,omitempty"`
	AssetDigest    string `json:"asset_digest,omitempty"`
	AssetSize      int64  `json:"asset_size,omitempty"`
}

func roots(bundlePath string) (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil { pool = x509.NewCertPool() }
	if bundlePath == "" { return pool, nil }
	pem, err := os.ReadFile(bundlePath)
	if err != nil { return nil, fmt.Errorf("cannot read bundled CA file: %w", err) }
	if !pool.AppendCertsFromPEM(pem) { return nil, errors.New("bundled CA file contains no usable certificates") }
	return pool, nil
}

func client(bundlePath string, timeout time.Duration) (*http.Client, error) {
	pool, err := roots(bundlePath); if err != nil { return nil, err }
	tr := &http.Transport{TLSClientConfig:&tls.Config{MinVersion:tls.VersionTLS12,RootCAs:pool},DisableKeepAlives:true,DisableCompression:true,ForceAttemptHTTP2:false,TLSNextProto:make(map[string]func(string,*tls.Conn) http.RoundTripper)}
	return &http.Client{Timeout:timeout,Transport:tr,CheckRedirect:func(req *http.Request, via []*http.Request) error { if len(via)>=8{return errors.New("too many redirects")}; if req.URL.Scheme!="https"{return errors.New("update redirect to non-HTTPS URL is blocked")}; req.Close=true; req.Header.Set("Connection","close"); return nil }}, nil
}

func parseVersion(v string) ([3]int, bool) {
	var out [3]int
	v=strings.TrimSpace(strings.TrimPrefix(strings.ToLower(v),"v")); if v==""||v=="dev"{return out,false}; if i:=strings.IndexAny(v,"+-");i>=0{v=v[:i]}
	parts:=strings.Split(v,"."); if len(parts)<2||len(parts)>3{return out,false}; for i,p:=range parts{n,err:=strconv.Atoi(p);if err!=nil||n<0{return out,false};out[i]=n}; return out,true
}
func IsNewer(current,latest string) bool { c,cok:=parseVersion(current);l,lok:=parseVersion(latest);if !cok||!lok{return false};for i:=0;i<len(c);i++{if l[i]!=c[i]{return l[i]>c[i]}};return false }

func Check(latestURL,currentVersion,assetName,bundlePath string,timeout time.Duration)(CheckResult,error){
	if latestURL==""{latestURL=DefaultLatestReleaseURL};if assetName==""{assetName=DefaultAssetName};c,err:=client(bundlePath,timeout);if err!=nil{return CheckResult{},err}
	req,err:=http.NewRequest(http.MethodGet,latestURL,nil);if err!=nil{return CheckResult{},err};req.Close=true;req.Header.Set("Connection","close");req.Header.Set("Accept","application/vnd.github+json");req.Header.Set("User-Agent","MailPush-for-Kindle/"+currentVersion)
	resp,err:=c.Do(req);if err!=nil{return CheckResult{},fmt.Errorf("cannot check GitHub release: %w",err)};defer resp.Body.Close();if resp.StatusCode!=http.StatusOK{return CheckResult{},fmt.Errorf("GitHub release check returned HTTP %d",resp.StatusCode)}
	var rel Release;if err:=json.NewDecoder(io.LimitReader(resp.Body,2*1024*1024)).Decode(&rel);err!=nil{return CheckResult{},fmt.Errorf("cannot parse GitHub release response: %w",err)}
	res:=CheckResult{CurrentVersion:currentVersion,LatestVersion:rel.TagName,ReleaseURL:rel.HTMLURL};if rel.Draft||rel.Prerelease||!IsNewer(currentVersion,rel.TagName){return res,nil}
	for _,a:=range rel.Assets{if a.Name==assetName{if !strings.HasPrefix(a.BrowserDownloadURL,"https://"){return CheckResult{},errors.New("release asset URL is not HTTPS")};res.Available=true;res.AssetURL=a.BrowserDownloadURL;res.AssetName=a.Name;res.AssetDigest=a.Digest;res.AssetSize=a.Size;return res,nil}}
	return CheckResult{},fmt.Errorf("release %s does not contain %s",rel.TagName,assetName)
}

// Legacy Go installer retained for compatibility/tests. Kindle UI updates no longer call
// this path; large download/extraction is handled by KOReader's own Lua/network stack.
func download(url,bundlePath,dst string,maxBytes int64,timeout time.Duration)error{c,err:=client(bundlePath,timeout);if err!=nil{return err};req,err:=http.NewRequest(http.MethodGet,url,nil);if err!=nil{return err};req.Close=true;req.Header.Set("Connection","close");req.Header.Set("Accept-Encoding","identity");resp,err:=c.Do(req);if err!=nil{return err};defer resp.Body.Close();f,err:=os.OpenFile(dst,os.O_CREATE|os.O_EXCL|os.O_WRONLY,0600);if err!=nil{return err};defer f.Close();n,err:=io.Copy(f,io.LimitReader(resp.Body,maxBytes+1));if err!=nil{return err};if n>maxBytes{return errors.New("update archive too large")};return f.Sync()}
func verifyDigest(path,digest string)error{if digest==""{return nil};const prefix="sha256:";if !strings.HasPrefix(strings.ToLower(digest),prefix){return errors.New("unsupported digest")};f,err:=os.Open(path);if err!=nil{return err};defer f.Close();h:=sha256.New();if _,err:=io.Copy(h,f);err!=nil{return err};if !strings.EqualFold(hex.EncodeToString(h.Sum(nil)),strings.TrimSpace(digest[len(prefix):])){return errors.New("update archive SHA-256 does not match GitHub release digest")};return nil}
func safeZipPath(name string)(string,error){name=strings.ReplaceAll(name,"\\","/");clean:=filepath.ToSlash(filepath.Clean(name));if clean=="."||clean=="mailpush.koplugin"{return clean,nil};if clean==".."||strings.HasPrefix(clean,"../")||!strings.HasPrefix(clean,"mailpush.koplugin/"){return "",errors.New("unsafe path")};return clean,nil}
func extract(zipPath,stagingRoot string)(string,error){zr,err:=zip.OpenReader(zipPath);if err!=nil{return "",err};defer zr.Close();for _,zf:=range zr.File{clean,err:=safeZipPath(zf.Name);if err!=nil{return "",err};dst:=filepath.Join(stagingRoot,filepath.FromSlash(clean));if zf.FileInfo().IsDir(){if err:=os.MkdirAll(dst,0755);err!=nil{return "",err};continue};if err:=os.MkdirAll(filepath.Dir(dst),0755);err!=nil{return "",err};r,err:=zf.Open();if err!=nil{return "",err};f,err:=os.OpenFile(dst,os.O_CREATE|os.O_EXCL|os.O_WRONLY,0644);if err!=nil{r.Close();return "",err};_,e:=io.Copy(f,r);f.Close();r.Close();if e!=nil{return "",e}};return filepath.Join(stagingRoot,"mailpush.koplugin"),nil}
func Install(pluginDir,assetURL,digest,bundlePath string,maxBytes int64,timeout time.Duration)error{if maxBytes<=0{maxBytes=DefaultMaxArchiveBytes};work,err:=os.MkdirTemp(filepath.Dir(pluginDir),".mailpush-update-");if err!=nil{return err};defer os.RemoveAll(work);archivePath:=filepath.Join(work,"update.zip");if err:=download(assetURL,bundlePath,archivePath,maxBytes,timeout);err!=nil{return err};if err:=verifyDigest(archivePath,digest);err!=nil{return err};staged,err:=extract(archivePath,filepath.Join(work,"unpacked"));if err!=nil{return err};backup:=pluginDir+".previous";_ = os.RemoveAll(backup);if err:=os.Rename(pluginDir,backup);err!=nil{return err};if err:=os.Rename(staged,pluginDir);err!=nil{_ = os.Rename(backup,pluginDir);return err};_ = os.RemoveAll(backup);return nil}
