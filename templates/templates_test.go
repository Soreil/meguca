package templates

import (
	"github.com/bakape/meguca/config"
	. "gopkg.in/check.v1"
	"html/template"
	"testing"
)

func Test(t *testing.T) { TestingT(t) }

type Templates struct{}

var _ = Suite(&Templates{})

func (t *Templates) TestBoardNavigation(c *C) {
	config.Config = config.Server{}
	config.Config.Boards.Enabled = []string{"a", "staff"}
	config.Config.Boards.Staff = "staff"
	config.Config.Boards.Psuedo = [][2]string{
		[2]string{"g", "https://google.com"},
	}
	html := boardNavigation()
	std := `<b id="navTop">[<a href="../a/">a</a> / <a href="../all/">all</a>` +
		` / <a href="https://google.com">g</a>]</b>`
	c.Assert(string(html), Equals, std)
}

func (t *Templates) TestBuildIndexTemplate(c *C) {
	v := vars{
		Config:     template.JS("c()"),
		ConfigHash: "a",
		Navigation: template.HTML("<hr>"),
	}
	source := `<script>{{.Config}}</script><b>{{.ConfigHash}}</b>` +
		`{{.Navigation}}<script>{{.IsMobile}}</script>`
	tmpl, err := template.New("index").Parse(source)
	c.Assert(err, IsNil)
	standard := Store{
		HTML: []byte("<script>c()</script><b>a</b><hr><script>false</script>"),
		Hash: "d99e2949415f7ec0",
	}
	res, err := buildIndexTemplate(tmpl, v, false)
	c.Assert(err, IsNil)
	c.Assert(res, DeepEquals, standard)
}

func (t *Templates) TestCompileTemplates(c *C) {
	templateRoot = "test"
	config.Config = config.Server{}
	config.ClientConfig = []byte{1}
	config.Config.Boards.Enabled = []string{"a"}
	standard := Store{
		HTML: []byte("<a></a>\n"),
		Hash: "eb51aca26e55050a",
	}
	defer func() {
		c.Assert(recover(), IsNil)
	}()
	Resources = Map{}
	c.Assert(Compile(), IsNil)
	c.Assert(Resources["index"], DeepEquals, standard)
	c.Assert(Resources["mobile"], DeepEquals, standard)
}
