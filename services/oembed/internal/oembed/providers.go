// Code generated from the ruby-oembed 0.17.0 builtin registry plus config/initializers/oembed.rb
// (Wistia, Sketchfab, FrameRate). Order matters: the first matching pattern wins, as in
// OEmbed::Providers.find. Facebook/Instagram are omitted because Rails never sets their
// required access_token, so the gem never matches them either.

package oembed

var builtinProviders = []Provider{
	{
		Endpoint: "https://my.matterport.com/api/v1/models/oembed/",
		Patterns: []string{
			`^https://([^\.]+\.)?matterport\.com/show/(.*?)`,
		},
	},
	{
		Endpoint: "https://www.tiktok.com/oembed",
		Patterns: []string{
			`^https://www\.tiktok\.com/(.*?)/video/(.*?)`,
		},
	},
	{
		Endpoint: "https://www.youtube.com/oembed/?scheme=https",
		Patterns: []string{
			`^http://([^\.]+\.)?youtube\.com/(.*?)`,
			`^https://([^\.]+\.)?youtube\.com/(.*?)`,
			`^http://([^\.]+\.)?youtu\.be/(.*?)`,
			`^https://([^\.]+\.)?youtu\.be/(.*?)`,
		},
	},
	{
		Endpoint: "https://codepen.io/api/oembed",
		Patterns: []string{
			`^http://codepen\.io/(.*?)`,
			`^https://codepen\.io/(.*?)`,
		},
	},
	{
		Endpoint: "https://www.flickr.com/services/oembed/",
		Patterns: []string{
			`^http://([^\.]+\.)?flickr\.com/(.*?)`,
			`^https://([^\.]+\.)?flickr\.com/(.*?)`,
			`^http://flic\.kr/(.*?)`,
			`^https://flic\.kr/(.*?)`,
		},
	},
	{
		Endpoint: "http://lab.viddler.com/services/oembed/",
		Patterns: []string{
			`^http://([^\.]+\.)?viddler\.com/(.*?)`,
		},
	},
	{
		Endpoint: "http://qik.com/api/oembed.{format}",
		Patterns: []string{
			`^http://qik\.com/(.*?)`,
			`^http://qik\.com/video/(.*?)`,
		},
	},
	{
		Endpoint: "http://revision3.com/api/oembed/",
		Patterns: []string{
			`^http://([^\.]+\.)?revision3\.com/(.*?)`,
		},
	},
	{
		Endpoint: "https://www.hulu.com/api/oembed.{format}",
		Patterns: []string{
			`^http://www\.hulu\.com/watch/(.*?)`,
			`^https://www\.hulu\.com/watch/(.*?)`,
		},
	},
	{
		Endpoint: "https://vimeo.com/api/oembed.{format}",
		Patterns: []string{
			`^http://([^\.]+\.)?vimeo\.com/(.*?)`,
			`^https://([^\.]+\.)?vimeo\.com/(.*?)`,
		},
	},
	{
		Endpoint: "https://publish.twitter.com/oembed",
		Patterns: []string{
			`^https://([^\.]+\.)?twitter\.com/(.*?)/status/(.*?)`,
		},
	},
	{
		Endpoint: "https://vine.co/oembed.{format}",
		Patterns: []string{
			`^http://([^\.]+\.)?vine\.co/v/(.*?)`,
			`^https://([^\.]+\.)?vine\.co/v/(.*?)`,
		},
	},
	{
		Endpoint: "https://www.slideshare.net/api/oembed/2",
		Patterns: []string{
			`^http://([^\.]+\.)?slideshare\.net/(.*?)/(.*?)`,
			`^https://([^\.]+\.)?slideshare\.net/(.*?)/(.*?)`,
			`^http://([^\.]+\.)?slideshare\.net/mobile/(.*?)/(.*?)`,
			`^https://([^\.]+\.)?slideshare\.net/mobile/(.*?)/(.*?)`,
		},
	},
	{
		Endpoint: "https://www.yfrog.com/api/oembed",
		Patterns: []string{
			`^http://yfrog\.com/(.*?)`,
		},
	},
	{
		Endpoint: "http://giphy.com/services/oembed",
		Patterns: []string{
			`^http://giphy\.com/(.*?)`,
			`^https://giphy\.com/(.*?)`,
		},
	},
	{
		Endpoint: "https://api.imgur.com/oembed.{format}",
		Patterns: []string{
			`^https://([^\.]+\.)?imgur\.com/gallery/(.*?)`,
			`^http://([^\.]+\.)?imgur\.com/gallery/(.*?)`,
		},
	},
	{
		Endpoint: "https://www.kickstarter.com/services/oembed",
		Patterns: []string{
			`^http://www\.kickstarter\.com/projects/(.*?)`,
			`^https://www\.kickstarter\.com/projects/(.*?)`,
		},
	},
	{
		Endpoint: "http://tv.majorleaguegaming.com/oembed",
		Patterns: []string{
			`^http://tv\.majorleaguegaming\.com/video/(.*?)`,
			`^http://mlg\.tv/video/(.*?)`,
		},
	},
	{
		Endpoint: "http://www.polleverywhere.com/services/oembed/",
		Patterns: []string{
			`^http://www\.polleverywhere\.com/polls/(.*?)`,
			`^http://www\.polleverywhere\.com/multiple_choice_polls/(.*?)`,
			`^http://www\.polleverywhere\.com/free_text_polls/(.*?)`,
		},
	},
	{
		Endpoint: "http://my.opera.com/service/oembed",
		Patterns: []string{
			`^http://my\.opera\.com/(.*?)`,
		},
	},
	{
		Endpoint: "http://widgets.clearspring.com/widget/v1/oembed/",
		Patterns: []string{
			`^http://www\.clearspring\.com/widgets/(.*?)`,
		},
	},
	{
		Endpoint: "http://www.nfb.ca/remote/services/oembed/",
		Patterns: []string{
			`^http://([^\.]+\.)?nfb\.ca/film/(.*?)`,
		},
	},
	{
		Endpoint: "https://www.scribd.com/services/oembed",
		Patterns: []string{
			`^http://([^\.]+\.)?scribd\.com/(.*?)`,
		},
	},
	{
		Endpoint: "https://speakerdeck.com/oembed.json",
		Patterns: []string{
			`^http://speakerdeck\.com/(.*?)/(.*?)`,
			`^https://speakerdeck\.com/(.*?)/(.*?)`,
		},
	},
	{
		Endpoint: "http://movieclips.com/services/oembed/",
		Patterns: []string{
			`^http://movieclips\.com/watch/(.*?)/(.*?)/`,
		},
	},
	{
		Endpoint: "http://www.23hq.com/23/oembed",
		Patterns: []string{
			`^http://www\.23hq\.com/(.*?)`,
		},
	},
	{
		Endpoint: "https://soundcloud.com/oembed",
		Patterns: []string{
			`^http://([^\.]+\.)?soundcloud\.com/(.*?)`,
			`^https://([^\.]+\.)?soundcloud\.com/(.*?)`,
		},
	},
	{
		Endpoint: "https://embed.spotify.com/oembed/",
		Patterns: []string{
			`^http://open\.spotify\.com/(.*?)`,
			`^https://open\.spotify\.com/(.*?)`,
			`^http://play\.spotify\.com/(.*?)`,
			`^https://play\.spotify\.com/(.*?)`,
			`^spotify\:(.*?)`,
		},
	},
	{
		Endpoint: "http://skitch.com/oembed",
		Patterns: []string{
			`^http://([^\.]+\.)?skitch\.com/(.*?)`,
			`^https://([^\.]+\.)?skitch\.com/(.*?)`,
		},
	},
	{
		Endpoint: "https://www.ted.com/talks/oembed.{format}",
		Patterns: []string{
			`^http://([^\.]+\.)?ted\.com/talks/(.*?)`,
			`^https://([^\.]+\.)?ted\.com/talks/(.*?)`,
		},
	},
	{
		Endpoint: "http://www.tumblr.com/oembed/1.0/",
		Patterns: []string{
			`^http://([^\.]+\.)?tumblr\.com/post/(.*?)`,
			`^https://([^\.]+\.)?tumblr\.com/post/(.*?)`,
		},
	},
	{
		Endpoint: "http://fast.wistia.com/oembed",
		Patterns: []string{
			`^http://([^\.]+\.)?wistia\.com/(.*?)`,
			`^http://([^\.]+\.)?wistia\.net/(.*?)`,
			`^https://([^\.]+\.)?wistia\.com/(.*?)`,
			`^https://([^\.]+\.)?wistia\.net/(.*?)`,
		},
	},
	{
		Endpoint: "https://sketchfab.com/oembed",
		Patterns: []string{
			`^http://sketchfab\.com/models/(.*?)`,
			`^https://sketchfab\.com/models/(.*?)`,
		},
	},
	{
		Endpoint: "https://framerate.tv/api/oembed",
		Patterns: []string{
			`^http://([^\.]+\.)?framerate\.tv/watch/(.*?)`,
			`^https://([^\.]+\.)?framerate\.tv/watch/(.*?)`,
		},
	},
}
