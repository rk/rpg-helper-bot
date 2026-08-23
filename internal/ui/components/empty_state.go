package components

import (
	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/widget"
)

func EmptyState(title, subtitle string) *primitives.BoxWidget {
	return primitives.VBox(
		primitives.Text(title).FontSize(18).Bold().Color(widget.RGBA8(80, 80, 80, 255)),
		primitives.Text(subtitle).FontSize(14).Color(widget.RGBA8(120, 120, 120, 255)),
	).Padding(24).Gap(8)
}

func Label(text string) *primitives.TextWidget {
	return primitives.Text(text).FontSize(13).Bold().Color(widget.RGBA8(70, 70, 70, 255))
}

func Muted(text string) *primitives.TextWidget {
	return primitives.Text(text).FontSize(12).Color(widget.RGBA8(110, 110, 110, 255))
}
