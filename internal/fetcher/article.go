package fetcher

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/hi20160616/exhtml"
	"github.com/hi20160616/gears"
	"github.com/hi20160616/ms-chinatimes/configs"
	"github.com/hycka/gocc"
	"github.com/pkg/errors"
	"golang.org/x/net/html"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Article struct {
	Id            string
	Title         string
	Content       string
	WebsiteId     string
	WebsiteDomain string
	WebsiteTitle  string
	UpdateTime    *timestamppb.Timestamp
	U             *url.URL
	raw           []byte
	doc           *html.Node
	requestTimes  int
}

var ErrTimeOverDays error = errors.New("article update time out of range")

func NewArticle() *Article {
	return &Article{
		WebsiteDomain: configs.Data.MS["chinatimes"].Domain,
		WebsiteTitle:  configs.Data.MS["chinatimes"].Title,
		WebsiteId:     fmt.Sprintf("%x", md5.Sum([]byte(configs.Data.MS["chinatimes"].Domain))),
	}
}

// List get all articles from database
func (a *Article) List() ([]*Article, error) {
	return load()
}

// Get read database and return the data by rawurl.
func (a *Article) Get(id string) (*Article, error) {
	as, err := load()
	if err != nil {
		return nil, err
	}

	for _, a := range as {
		if a.Id == id {
			return a, nil
		}
	}
	return nil, fmt.Errorf("[%s] no article with id: %s",
		configs.Data.MS["chinatimes"].Title, id)
}

func (a *Article) Search(keyword ...string) ([]*Article, error) {
	as, err := load()
	if err != nil {
		return nil, err
	}

	as2 := []*Article{}
	for _, a := range as {
		for _, v := range keyword {
			v = strings.ToLower(strings.TrimSpace(v))
			switch {
			case a.Id == v:
				as2 = append(as2, a)
			case a.WebsiteId == v:
				as2 = append(as2, a)
			case strings.Contains(strings.ToLower(a.Title), v):
				as2 = append(as2, a)
			case strings.Contains(strings.ToLower(a.Content), v):
				as2 = append(as2, a)
			case strings.Contains(strings.ToLower(a.WebsiteDomain), v):
				as2 = append(as2, a)
			case strings.Contains(strings.ToLower(a.WebsiteTitle), v):
				as2 = append(as2, a)
			}
		}
	}
	return as2, nil
}

type ByUpdateTime []*Article

func (u ByUpdateTime) Len() int      { return len(u) }
func (u ByUpdateTime) Swap(i, j int) { u[i], u[j] = u[j], u[i] }
func (u ByUpdateTime) Less(i, j int) bool {
	return u[i].UpdateTime.AsTime().Before(u[j].UpdateTime.AsTime())
}

var timeout = func() time.Duration {
	t, err := time.ParseDuration(configs.Data.MS["chinatimes"].Timeout)
	if err != nil {
		log.Printf("[%s] timeout init error: %v",
			configs.Data.MS["chinatimes"].Title, err)
		return time.Duration(1 * time.Minute)
	}
	return t
}()

func (a *Article) dail(u string) (*Article, error) {
	var err error
	a.U, err = url.Parse(u)
	if err != nil {
		return nil, err
	}
	a.raw, a.doc, err = exhtml.GetRawAndDoc(a.U, 1*time.Minute)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, errors.WithMessagef(err,
				"404 on url: %s", u)
			// if a.requestTimes == 5 {
			// }
			// a.requestTimes++
			// return a.dail(u)
		}
		if strings.Contains(err.Error(), "invalid header") {
			a.Title = a.U.Path
			a.UpdateTime = timestamppb.Now()
			a.Content, err = a.fmtContent("")
			if err != nil {
				return nil, err
			}
			return a, nil
		} else {
			return nil, err
		}
	}
	return a, err
}

// fetchArticle fetch article by rawurl
func (a *Article) fetchArticle(rawurl string) (*Article, error) {
	translate := func(x string, err error) (string, error) {
		if err != nil {
			return "", err
		}
		tw2s, err := gocc.New("tw2s")
		if err != nil {
			return "", err
		}
		return tw2s.Convert(x)
	}

	var err error
	a, err = a.dail(rawurl)
	if err != nil {
		return nil, err
	}

	a.Id = fmt.Sprintf("%x", md5.Sum([]byte(rawurl)))

	a.Title, err = a.fetchTitle()
	if err != nil {
		return nil, err
	}

	a.UpdateTime, err = a.fetchUpdateTime()
	if err != nil {
		return nil, err
	}

	// content should be the last step to fetch
	a.Content, err = a.fetchContent()
	if err != nil {
		return nil, err
	}

	a.Content, err = translate(a.fmtContent(a.Content))
	if err != nil {
		return nil, err
	}
	return a, nil

}

func (a *Article) fetchTitle() (string, error) {
	n := exhtml.ElementsByTag(a.doc, "title")
	if n == nil || len(n) == 0 {
		return "", fmt.Errorf("[%s] getTitle error, there is no element <title>",
			configs.Data.MS["chinatimes"].Title)
	}
	title := n[0].FirstChild.Data
	rp := strings.NewReplacer(" ｜ 蘋果新聞網 ｜ 蘋果日報", "")
	title = strings.TrimSpace(rp.Replace(title))
	return gears.ChangeIllegalChar(title), nil
}

func (a *Article) fetchUpdateTime() (*timestamppb.Timestamp, error) {
	if a.raw == nil {
		return nil, errors.Errorf("[%s] fetchUpdateTime: raw is nil: %s",
			configs.Data.MS["chinatimes"].Title, a.U.String())
	}

	t := time.Now() // if no time fetched, return current time
	var err error
	n := exhtml.MetasByProperty(a.doc, "article:published_time")
	if len(n) == 0 {
		return nil, fmt.Errorf("[%s] fetchUpdateTime error, no meta named date matched: %s",
			configs.Data.MS["chinatimes"].Title, a.U.String())
	}
	for _, nn := range n {
		for _, x := range nn.Attr {
			if x.Key == "content" {
				t, err = time.Parse(time.RFC3339, x.Val)
				if err != nil {
					return nil, errors.WithMessage(err,
						"caught meta but no content matched.")
				}
			}
		}
	}

	if t.Before(time.Now().AddDate(0, 0, -3)) {
		return timestamppb.New(t), ErrTimeOverDays
	}
	return timestamppb.New(t), err
}

func shanghai(t time.Time) time.Time {
	loc := time.FixedZone("UTC", 8*60*60)
	return t.In(loc)
}

func (a *Article) fetchContent() (string, error) {
	if a.doc == nil {
		return "", errors.Errorf("[%s] fetchContent: doc is nil: %s", configs.Data.MS["chinatimes"].Title, a.U.String())
	}
	body := ""
	bodyN := exhtml.ElementsByTagAndId(a.doc, "script", "fusion-metadata")
	if len(bodyN) == 0 {
		return body, errors.Errorf("no article content matched: %s", a.U.String())
	}
	// Fetch content
	bodyJs := func() string {
		for _, v := range bodyN {
			if v.FirstChild != nil && v.FirstChild.Type == html.TextNode {
				return v.FirstChild.Data
			}
		}
		return ""
	}()

	re := regexp.MustCompile(`(?m)Fusion\.globalContent=(?P<x>.*?);Fusion.globalContentConfig={"source":"`)
	if !re.MatchString(bodyJs) {
		return "", fmt.Errorf("nil content matched: %s", a.U.String())
	}
	rs := re.FindStringSubmatch(bodyJs)
	c := struct {
		Content_elements []struct {
			Content string `json:"content"`
		} `json:"content_elements"`
	}{}
	if err := json.Unmarshal([]byte(rs[1]), &c); err != nil {
		return "", err
	}

	for _, v := range c.Content_elements {
		re := regexp.MustCompile(`(?m)<mark .*?>(?P<x>.*?)</mark>`)
		x := re.ReplaceAllString(v.Content, "${x}")
		re = regexp.MustCompile(`(?m)<b>(?P<x>.*?)</b>`)
		x = re.ReplaceAllString(x, "**${x}**")
		re = regexp.MustCompile(`(?m)<a href="(?P<href>.*?)">(?P<x>.*?)</a>`)
		x = re.ReplaceAllString(x, "[${x}](${href})")
		x = strings.ReplaceAll(x, "「", "“")
		x = strings.ReplaceAll(x, "」", "”")
		body += x + "  \n"
	}
	return body, nil
}

func (a *Article) fmtContent(body string) (string, error) {
	var err error
	title := "# " + a.Title + "\n\n"
	lastupdate := shanghai(a.UpdateTime.AsTime()).Format(time.RFC3339)
	webTitle := fmt.Sprintf(" @ [%s](/list/?v=%[1]s): [%[2]s](http://%[2]s)", a.WebsiteTitle, a.WebsiteDomain)
	u, err := url.QueryUnescape(a.U.String())
	if err != nil {
		u = a.U.String() + "\n\nunescape url error:\n" + err.Error()
	}
	body = title +
		"LastUpdate: " + lastupdate +
		webTitle + "\n\n" +
		"---\n" +
		body + "\n\n" +
		"原地址：" + fmt.Sprintf("[%s](%[1]s)", strings.Split(u, "?tmpl=")[0])
	return body, nil
}
