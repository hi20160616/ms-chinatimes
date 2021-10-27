package fetcher

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/hi20160616/exhtml"
	"github.com/pkg/errors"
)

func TestFetchTitle(t *testing.T) {
	tests := []struct {
		url   string
		title string
	}{
		{"https://tw.chinatimes.com/local/20211015/EJNCNTZMQVE5TE2PHRHB5MN3HY/", "K律師論點｜城中城大火若是人禍　究責關鍵先釐清這事"},
		{"https://tw.chinatimes.com/politics/20211012/GTRJUAUOXBAUXIIQZEAR72XHSI/", "網傳綠營大老舔共文　讚「兩岸只有親情和恩情」？當事人說話了"},
	}
	for _, tc := range tests {
		a := NewArticle()
		u, err := url.Parse(tc.url)
		if err != nil {
			t.Error(err)
		}
		a.U = u
		// Dail
		a.raw, a.doc, err = exhtml.GetRawAndDoc(a.U, timeout)
		if err != nil {
			t.Error(err)
		}
		got, err := a.fetchTitle()
		if err != nil {
			if !errors.Is(err, ErrTimeOverDays) {
				t.Error(err)
			} else {
				fmt.Println("ignore pass test: ", tc.url)
			}
		} else {
			if tc.title != got {
				t.Errorf("\nwant: %s\n got: %s", tc.title, got)
			}
		}
	}

}

func TestFetchUpdateTime(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://tw.chinatimes.com/local/20211015/EJNCNTZMQVE5TE2PHRHB5MN3HY/", "2021-10-15 16:13:47.476 +0800 UTC"},
		{"https://tw.chinatimes.com/politics/20211012/GTRJUAUOXBAUXIIQZEAR72XHSI/", "2021-10-12 20:36:20.657 +0800 UTC"},
	}
	var err error
	for _, tc := range tests {
		a := NewArticle()
		a.U, err = url.Parse(tc.url)
		if err != nil {
			t.Error(err)
		}
		// Dail
		a.raw, a.doc, err = exhtml.GetRawAndDoc(a.U, timeout)
		if err != nil {
			t.Error(err)
		}
		tt, err := a.fetchUpdateTime()
		if err != nil {
			if !errors.Is(err, ErrTimeOverDays) {
				t.Error(err)
			}
		}
		ttt := tt.AsTime()
		got := shanghai(ttt)
		if got.String() != tc.want {
			t.Errorf("\nwant: %s\n got: %s", tc.want, got.String())
		}
	}
}

func TestFetchContent(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://tw.chinatimes.com/property/20211021/2ICEHGCCWRFNXOLYWGGHGEIJDA/", "K律師論點｜城中城大火若是人禍　究責關鍵先釐清這事"},
		{"https://tw.chinatimes.com/local/20211015/EJNCNTZMQVE5TE2PHRHB5MN3HY/", "K律師論點｜城中城大火若是人禍　究責關鍵先釐清這事"},
		{"https://tw.chinatimes.com/politics/20211012/GTRJUAUOXBAUXIIQZEAR72XHSI/", "網傳綠營大老舔共文　讚「兩岸只有親情和恩情」？當事人說話了"},
		{"https://tw.chinatimes.com/property/20211025/SARWV3SGOZALZJVUMBLTSM6EYI/", ""},
	}
	var err error

	for _, tc := range tests {
		a := NewArticle()
		a.U, err = url.Parse(tc.url)
		if err != nil {
			t.Error(err)
		}
		// Dail
		a.raw, a.doc, err = exhtml.GetRawAndDoc(a.U, timeout)
		if err != nil {
			t.Error(err)
		}
		c, err := a.fetchContent()
		if err != nil {
			t.Error(err)
		}
		fmt.Println(c)
	}
}

func TestFetchArticle(t *testing.T) {
	tests := []struct {
		url string
		err error
	}{
		{"https://tw.chinatimes.com/local/20211015/EJNCNTZMQVE5TE2PHRHB5MN3HY/", ErrTimeOverDays},
		{"https://tw.chinatimes.com/property/20211021/2ICEHGCCWRFNXOLYWGGHGEIJDA/", ErrTimeOverDays},
		{"https://tw.chinatimes.com/property/20211025/SARWV3SGOZALZJVUMBLTSM6EYI/", nil},
	}
	for _, tc := range tests {
		a := NewArticle()
		a, err := a.fetchArticle(tc.url)
		if err != nil {
			if !errors.Is(err, ErrTimeOverDays) {
				t.Error(err)
			} else {
				fmt.Println("ignore old news pass test: ", tc.url)
			}
		} else {
			fmt.Println("pass test: ", a.Content)
		}
	}
}
