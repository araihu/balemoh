package view

type dashboardSection struct {
	ID, Title string
	Widgets   []Widget
}

// Group by source, independently of each widget's chosen renderer.
func dashboardSections(widgets []Widget) []dashboardSection {
	sections := []dashboardSection{{ID: "hosts", Title: "Hosts"}, {ID: "services", Title: "Services"}, {ID: "other", Title: "Widgets"}}
	for _, widget := range widgets {
		index := 2
		switch widget.SourceKind {
		case "host":
			index = 0
		case "service":
			index = 1
		}
		sections[index].Widgets = append(sections[index].Widgets, widget)
	}
	result := make([]dashboardSection, 0, len(sections))
	for _, section := range sections {
		if len(section.Widgets) > 0 {
			result = append(result, section)
		}
	}
	return result
}
