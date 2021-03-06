/*
 Image thumbnail HTML rendering
*/

import {config} from '../../state'
import {escape} from 'underscore'
import options from '../../options'
import {
	parseHTML, commaList, parseAttributes, ElementAttributes
} from '../../util'
import {ImageData, fileTypes} from '../models'
import {images as lang} from '../../lang'

// Render a thumbnail of an image, according to configuration settings
export function renderImage(data: ImageData, reveal?: boolean): string {
	const showThumb = options.get("thumbs") !== 'hide' || reveal
	return parseHTML
		`<figure>
			${renderFigcaption(data, reveal)}
			${config.images.hats && showThumb ? '<span class="hat"></span>': ''}
			${showThumb ? renderThumbnail(data) : ''}
		</figure>`
}

// Render the information caption above the image
export function renderFigcaption(data: ImageData, reveal: boolean): string {
	const list = commaList([
		data.audio ? '\u266B' : '',
		data.length.toString(),
		readableFilesize(data.size),
		`${data.dims[0]}x${data.dims[1]}`,
		data.apng ? 'APNG' : ''
	])
	return parseHTML
		`<figcaption>
			${hiddenToggle(reveal)}
			${imageSearch(data)}
			<span>
				(${list})
			</span>
			${imageLink(data)}
		</figcaption>`
}

// Renders a human readable file size string
function readableFilesize(size: number): string {
	if (size < 1024) {
		return size + ' B'
	}
	if (size < 1048576) {
		return Math.round(size / 1024) + ' KB'
	}
	const text = Math.round(size / 104857.6).toString()
	return `${text.slice(0, -1)}.${text.slice(-1)} MB`
}

// Render the button for toggling hidden thumbnails
function hiddenToggle(reveal: boolean): string {
	if (options.get('thumbs') !== 'hide') {
		return ''
	}
	return parseHTML
		`<a class="imageToggle">
			[${lang[reveal ? 'hide' : 'show']}]
		</a>`
}

// Base URLs of image addresses
const imagePaths: {[type: string]: string} = {
	src: '/img/src/',
	thumb: '/img/thumb/',
	mid: '/img/mid/',
	spoil: '/ass/spoil/spoiler'
}

type ISTemplate = (data: ImageData) => string

// Generate template functions for each image search engine
const searchTemplates = (function() {
	const models = [
		{
			engine: 'google',
			url: 'https://www.google.com/searchbyimage?image_url=',
			type: 'thumb',
			symbol: 'G'
		},
		{
			engine: 'iqdb',
			url: 'http://iqdb.org/?url=',
			type: 'thumb',
			symbol: 'Iq'
		},
		{
			engine: 'saucenao',
			url: 'http://saucenao.com/search.php?db=999&url=',
			type: 'thumb',
			symbol: 'Sn'
		},
		{
			engine: 'desustorage',
			type: 'MD5',
			url: 'https://desustorage.org/_/search/image/',
			symbol: 'Ds'
		},
		{
			engine: 'exhentai',
			type: 'SHA1',
			url: 'http://exhentai.org/?fs_similar=1&fs_exp=1&f_shash=',
			symbol: 'Ex'
		}
	]

	const templates: {[engine: string]: ISTemplate} = {}
	for (let {engine, url, type, symbol} of models) {
		const attrs: ElementAttributes = {
			target: '_blank',
			rel: 'nofollow',
			class: 'imageSearch ' + engine
		}
		templates[engine] = data => {
			if (!options.get(engine)) {
				return ''
			}
			attrs['href'] = url
				+ (type === 'thumb' ? thumbPath(data, false) : data[type])
			return parseHTML
				`<a ${parseAttributes(attrs)}>
					${symbol}
				</a>`
		}
	}

	return templates
})()

// Render image search links in accordance to client settings
function imageSearch(data: ImageData): string {
	let html = ''

	// Only render google for PDFs
	if (data.fileType === fileTypes.pdf) {
		if (options.get("google")) {
			return searchTemplates['google'](data)
		}
		return ''
	}
	for (let engine in searchTemplates) {
		html += searchTemplates[engine](data)
	}
	return html
}

// Get the thumbnail path of an image, accounting for not thumbnail of specific
// type being present
function thumbPath(data: ImageData, mid: boolean): string {
	const type = mid ? 'mid' : 'thumb'
	let ext: string
	switch (data.fileType) {
	case fileTypes.mp4:
	case fileTypes.jpeg:
		ext = '.jpg'
		break
	case fileTypes.png:
	case fileTypes.gif:
	case fileTypes.webm:
	case fileTypes.pdf:
	case fileTypes.mp3:
	case fileTypes.ogg:
		ext = '.png'
		break
	}
	return imagePaths[type] + data.file + ext
}

// Resolve the path to the source file of an upload
function sourcePath({file, fileType}: ImageData): string {
	return imagePaths['src'] + file + sourceExtension(fileType)
}

// Resolve the extension of the source file
function sourceExtension(fileType: fileTypes): string {
	const extensions: {[type: number]: string} = {
		[fileTypes.jpeg]: '.jpg',
		[fileTypes.png]: '.png',
		[fileTypes.gif]: '.gif',
		[fileTypes.webm]: '.webm',
		[fileTypes.pdf]: '.pdf',
		[fileTypes.svg]: '.svg',
		[fileTypes.mp4]: '.mp4',
		[fileTypes.mp3]: '.mp3',
		[fileTypes.ogg]: '.ogg'
	}
	return extensions[fileType]
}

// Render a name + download link of an image
function imageLink(data: ImageData): string {
	let name = '',
		{file, fileType, imgnm} = data
	const m = imgnm.match(/^(.*)\.\w{3,4}$/)
	if (m) {
		name = m[1]
	}
	const fullName = escape(imgnm),
		tooLong = name.length >= 38
	if (tooLong) {
		imgnm = escape(name.slice(0, 30))
			+ '(&hellip;)'
			+ escape(sourceExtension(fileType))
	}
	const attrs: ElementAttributes = {
		href: sourcePath(data),
		rel: 'nofollow',
		download: fullName
	}
	if (tooLong) {
		attrs['title'] = fullName
	}
	return parseHTML
		`<a ${parseAttributes(attrs)}>
			${imgnm}
		</a>`
}

// Render a hat on top of the thumbnail, if enabled
function renderHat(showThumb: boolean): string {
	if (showThumb && config.images.hats) {
		return '<span class="hat"></span>'
	}
	return ''
}

// Render the actual thumbnail image
export function renderThumbnail(data: ImageData, href?: string): string {
	const src = sourcePath(data)
	let thumb: string,
		[width, height, thumbWidth, thumbHeight] = data.dims

	if (data.spoiler && options.get('spoilers')) {
		// Spoilered and spoilers enabled
		thumb = imagePaths['spoil'] + data.spoiler + '.jpg'
		thumbWidth = thumbHeight = 250
	} else if (data.fileType === fileTypes.gif && options.get('autogif')) {
		// Animated GIF thumbnails
		thumb = src
	} else {
		thumb = thumbPath(data, options.get('thumbs') !== 'small')
	}

	const linkAttrs: ElementAttributes = {
		target: '_blank',
		rel: 'nofollow',
		href: href || src
	}
	const imgAttrs: ElementAttributes = {
		src: thumb,
		width: thumbWidth.toString(),
		height: thumbHeight.toString()
	}

	// Catalog pages
	if (href) {
		// Handle the thumbnails with the HTML5 History controller
		linkAttrs['class'] = 'history'

		// No image hover previews
		imgAttrs['class'] = 'expanded'
		if(options.get('thumbs') === 'hide') {
			imgAttrs['style'] = 'display: none'
		}
	}

	return parseHTML
		`<a ${parseAttributes(linkAttrs)}>
			<img ${parseAttributes(imgAttrs)}>
		</a>`
}
