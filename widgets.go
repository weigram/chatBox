package main

import "github.com/egoist/mygo/ui"

// The app's icons, drawn as SVGs in the color of the text around them:
// shapes only, since MyGo draws no text inside an SVG.

// pinIcon is the pin of the window's top right corner: the pushpin the
// app is given, its shapes in the color of the text around it.
var pinIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 1024 1024">
  <path fill="currentColor" d="M878.3 392.1L631.9 145.7c-6.5-6.5-15-9.7-23.5-9.7s-17 3.2-23.5 9.7L423.8 306.9c-12.2-1.4-24.5-2-36.8-2-73.2 0-146.4 24.1-206.5 72.3-15.4 12.3-16.7 35.4-2.7 49.4l181.7 181.7-215.4 215.2c-2.6 2.6-4.3 6.1-4.6 9.8l-3.4 37.2c-0.9 9.4 6.6 17.4 15.9 17.4 0.5 0 1 0 1.5-0.1l37.2-3.4c3.7-0.3 7.2-2 9.8-4.6l215.4-215.4 181.7 181.7c6.5 6.5 15 9.7 23.5 9.7 9.7 0 19.3-4.2 25.9-12.4 56.3-70.3 79.7-158.3 70.2-243.4l161.1-161.1c12.9-12.8 12.9-33.8 0-46.8zM666.2 549.3l-24.5 24.5 3.8 34.4c3.7 33.7 1 67.2-8.2 99.7-5.4 19-12.8 37.1-22.2 54.2L262 408.8c12.9-7.1 26.3-13.1 40.3-17.9 27.2-9.4 55.7-14.1 84.7-14.1 9.6 0 19.3 0.5 28.9 1.6l34.4 3.8 24.5-24.5L608.5 224 800 415.5 666.2 549.3z"/>
</svg>`))

// plusIcon adds a chat: the dock's and the tab strip's new-button.
var plusIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24">
  <path d="M12 4.6 L12 19.4 M4.6 12 L19.4 12"
        stroke="currentColor" stroke-width="1.9" stroke-linecap="round"/>
</svg>`))

// gearIcon is the dock's settings button: a cog, its ring and the teeth
// standing on it.
var gearIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24">
  <circle cx="12" cy="12" r="5.3" fill="none" stroke="currentColor" stroke-width="2.8"/>
  <g fill="currentColor">
    <g transform="rotate(0 12 12)"><rect x="9.9" y="1.5" width="4.2" height="3.6" rx="1.1"/></g>
    <g transform="rotate(60 12 12)"><rect x="9.9" y="1.5" width="4.2" height="3.6" rx="1.1"/></g>
    <g transform="rotate(120 12 12)"><rect x="9.9" y="1.5" width="4.2" height="3.6" rx="1.1"/></g>
    <g transform="rotate(180 12 12)"><rect x="9.9" y="1.5" width="4.2" height="3.6" rx="1.1"/></g>
    <g transform="rotate(240 12 12)"><rect x="9.9" y="1.5" width="4.2" height="3.6" rx="1.1"/></g>
    <g transform="rotate(300 12 12)"><rect x="9.9" y="1.5" width="4.2" height="3.6" rx="1.1"/></g>
  </g>
</svg>`))

// editIcon is the pencil of a chat's edit button: a nib drawing a line.
var editIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24">
  <path fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round"
    stroke-linejoin="round" d="M4 20h4L19.5 8.5a2.1 2.1 0 0 0-3-3L5 17v3z"/>
  <path fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round"
    d="M14.5 6.5l3 3"/>
</svg>`))

// proxyIcon marks a chat whose requests take a proxy of their own, or go
// straight out: a globe-like ring with a bolt through it. Its shapes take
// the color of the text around them, as every icon does.
var proxyIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 1024 1024">
  <path fill="currentColor" d="M512 85.333333c235.648 0 426.666667 191.018667 426.666667 426.666667s-191.018667 426.666667-426.666667 426.666667S85.333333 747.648 85.333333 512 276.352 85.333333 512 85.333333z m0 85.333334a341.333333 341.333333 0 1 0 0 682.666666 341.333333 341.333333 0 0 0 0-682.666666z m-7.936 72.704c6.698667 1.792 7.765333 13.653333 7.893333 36.906666L512 413.866667c0 10.069333 0 15.104 3.114667 18.218666 2.602667 2.56 6.528 3.029333 13.610666 3.114667h49.962667c65.28 0 97.877333 0 110.208 21.205333 12.373333 21.205333-3.754667 49.578667-36.010667 106.282667l-100.992 177.792c-16 28.16-24.021333 42.24-31.957333 40.106667C512 778.538667 512 762.368 512 729.984V610.133333c0-10.069333 0-15.104-3.114667-18.218666-3.114667-3.114667-8.149333-3.114667-18.218666-3.114667H424.746667c-51.669333-0.256-78.677333-2.346667-89.642667-21.205333-12.373333-21.205333 3.754667-49.578667 36.010667-106.282667l100.992-177.792c16-28.16 24.021333-42.24 31.957333-40.106667z"/>
</svg>`))

// moreIcon is the toolbar's "more" menu: three dots, no chevron.
var moreIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24">
  <circle cx="5.2" cy="12" r="1.95" fill="currentColor"/>
  <circle cx="12" cy="12" r="1.95" fill="currentColor"/>
  <circle cx="18.8" cy="12" r="1.95" fill="currentColor"/>
</svg>`))

// The page controls of the toolbar: back, forward and reload.
var (
	backIcon    = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><path d="M14.6 5.4 L8 12 L14.6 18.6" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/></svg>`))
	forwardIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><path d="M9.4 5.4 L16 12 L9.4 18.6" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/></svg>`))
	reloadIcon  = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><path d="M19.6 12a7.6 7.6 0 1 1-2.3-5.4" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"/><path d="M18.4 3.2 L18.6 7.4 L14.4 7.6" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/></svg>`))
)

// closeIcon closes a tab.
var closeIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24">
  <path d="M6.4 6.4 L17.6 17.6 M17.6 6.4 L6.4 17.6"
        stroke="currentColor" stroke-width="2.1" stroke-linecap="round"/>
</svg>`))

// smallIconButton builds a bare icon button of the given size, as the tab
// strip's new-tab and close buttons are: no face until the pointer is on
// it.
func smallIconButton(c *ui.Context, label string, icon *ui.SVG, size float32, clicked func()) {
	t := c.Theme()
	b := ui.Row(c).Size(size, size).Radius(t.Radius).AlignItems(ui.Center).Justify(ui.Center)
	b.Label(label)
	b.Tooltip(label)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	if b.Clicked() && clicked != nil {
		clicked()
	}
	b.Children(func() {
		ui.Icon(c, icon).Size(size*0.52, size*0.52).TextColor(t.TextMuted)
	})
}

// iconButton builds a button of one icon with a face of its own, as the
// toolbar's: hovered gray, and the accent while it is on.
func iconButton(c *ui.Context, label string, icon *ui.SVG, on bool, clicked func()) {
	iconButtonOn(c, label, icon, on, true, clicked)
}

// dockIconButton builds one of the dock's bottom buttons: a bare icon,
// which shows a face only under the pointer, as Cherry Studio's do.
func dockIconButton(c *ui.Context, label string, icon *ui.SVG, clicked func()) {
	iconButtonOn(c, label, icon, false, false, clicked)
}

// iconButtonOn is the two of them: face says whether the button wears the
// face of its row when nothing touches it.
func iconButtonOn(c *ui.Context, label string, icon *ui.SVG, on, face bool, clicked func()) {
	iconButtonSized(c, label, icon, on, face, 40, 34, 17, clicked)
}

// iconButtonSized is iconButtonOn with the sizes of its own, for the
// toolbar's page controls.
func iconButtonSized(c *ui.Context, label string, icon *ui.SVG, on, face bool,
	w, h, iconSize float32, clicked func()) {
	t := c.Theme()
	b := ui.Row(c).Width(w).Height(h).Radius(t.Radius + 4).
		AlignItems(ui.Center).Justify(ui.Center)
	b.Label(label)
	b.Tooltip(label)
	switch {
	case on:
		b.Background(t.Accent)
	case b.Hovered():
		b.Background(t.SurfaceHover)
	case face:
		b.Background(t.Surface)
	}
	if b.Clicked() && clicked != nil {
		clicked()
	}
	b.Children(func() {
		pic := ui.Icon(c, icon).Size(iconSize, iconSize)
		if on {
			pic.TextColor(t.AccentText)
		} else {
			pic.TextColor(t.Text)
		}
	})
}

// circleIconButton builds a round icon button, as the pin at the window's
// top right corner is: dark gray and filled while it is on, a light circle
// under the pointer, and nothing otherwise.
func circleIconButton(c *ui.Context, label string, icon *ui.SVG, on bool, size, iconSize float32, clicked func()) {
	t := c.Theme()
	b := ui.Row(c).Size(size, size).Radius(size / 2).AlignItems(ui.Center).Justify(ui.Center)
	b.Label(label)
	b.Tooltip(label)
	switch {
	case on:
		// The white pill a chosen tab wears, as Cherry Studio draws it.
		b.Background(t.Background)
	case b.Hovered():
		b.Background(hoverWash(t))
	}
	if b.Clicked() && clicked != nil {
		clicked()
	}
	b.Children(func() {
		pic := ui.Icon(c, icon).Size(iconSize, iconSize)
		if on {
			pic.TextColor(t.Text)
		} else {
			pic.TextColor(t.TextMuted)
		}
	})
}

// hoverWash is the wash of white over a strip, for what the pointer rests
// on, thin enough that the strip still shows through.
func hoverWash(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGBA(0xFF, 0xFF, 0xFF, 0.08)
	}
	return ui.RGBA(0xFF, 0xFF, 0xFF, 0.65)
}

// chip is the pill of a chosen tab or icon, which lifts off the strip it
// sits on: white on the light theme's gray, a lighter surface on the dark
// theme's, as Chrome and Cherry Studio draw the tab chosen.
func chip(t *ui.Theme) ui.Color {
	if t.Dark {
		return t.SurfaceHover
	}
	return t.Background
}

// iconChip is the circle an icon of the dock sits on, the chosen one's the
// brightest and the largest.
func iconChip(t *ui.Theme, chosen, hovered bool) ui.Color {
	alpha := float32(0)
	switch {
	case chosen:
		if t.Dark {
			alpha = 0.14
		} else {
			alpha = 1
		}
	case hovered:
		if t.Dark {
			alpha = 0.10
		} else {
			alpha = 0.95
		}
	default:
		if t.Dark {
			alpha = 0.05
		} else {
			alpha = 0.70
		}
	}
	return ui.RGBA(0xFF, 0xFF, 0xFF, alpha)
}

// pageButton builds one of the toolbar's page controls: an icon button
// that grays out when the page cannot do it.
func pageButton(c *ui.Context, label string, icon *ui.SVG, enabled bool, clicked func()) {
	t := c.Theme()
	b := ui.Row(c).Width(34).Height(30).Radius(t.Radius + 4).
		AlignItems(ui.Center).Justify(ui.Center)
	b.Label(label)
	b.Tooltip(label)
	// 平时没有底色，只在指针停上去时浮出来一层浅灰，和 Cherry Studio 一致。
	switch {
	case !enabled:
	case b.Pressed():
		b.Background(t.SurfaceHover)
	case b.Hovered():
		b.Background(t.Surface)
	}
	if enabled && b.Clicked() && clicked != nil {
		clicked()
	}
	b.Children(func() {
		pic := ui.Icon(c, icon).Size(18, 18)
		if enabled {
			pic.TextColor(t.Text)
		} else {
			pic.TextColor(t.TextMuted)
		}
	})
}
