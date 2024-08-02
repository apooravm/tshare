package shared

import (
	"fmt"
	"math/rand"
)

type ProgressBar struct {
	OngoingFileSize            int
	OngoingFileTransferredSize int
	OngoingFileBlip            int

	TotalTransferSize    int
	TotalTransferredSize int
	TotalTransferBlip    int

	AllTransferStarted bool
	TransferStarted    bool
	// single, total
	Type string

	BarLength int
	RgbOn     bool
	// Display progress in MB or kb
	InMB        bool
	SizeConvDiv float64
	SizeUnit    string
	// words like sent/received
	TrailingText string
	Colours      []string
	IsOff        bool
}

func NewProgressBar(totalFileSize int, pbType string, barLen int, rgbOn bool, trailingText string, inMB bool, isOff bool) *ProgressBar {
	var SizeConvDiv float64 = 1000
	var sizeUnit string = "kb"
	if inMB {
		SizeConvDiv = 1000_000
		sizeUnit = "MB"
	}
	return &ProgressBar{
		TotalTransferSize: totalFileSize,
		TotalTransferBlip: totalFileSize / barLen,
		Type:              pbType,
		BarLength:         barLen,
		RgbOn:             rgbOn,
		TrailingText:      trailingText,
		InMB:              inMB,
		SizeConvDiv:       SizeConvDiv,
		SizeUnit:          sizeUnit,
		Colours:           []string{"red", "yellow", "magenta", "green", "cyan"},
		IsOff:             isOff,
	}
}

func (pb *ProgressBar) PrintPostDoneMessage(message string) {
	if pb.IsOff {
		return
	}

	if pb.Type == "single" {
		fmt.Println(message)
	}
}

// Reset individual values for the new file.
func (pb *ProgressBar) UpdateOngoingForNewFile(filesize int) {
	pb.TransferStarted = false
	pb.OngoingFileTransferredSize = 0

	pb.OngoingFileSize = filesize
	pb.OngoingFileBlip = filesize / pb.BarLength

	if pb.Type == "total" {
		pb.TransferStarted = true
	}
}

// Update with the size of the recent chunk transferred
func (pb *ProgressBar) UpdateTransferredSize(chunkSize int) {
	pb.OngoingFileTransferredSize += chunkSize
	pb.TotalTransferredSize += chunkSize
}

func (pb *ProgressBar) Show() {
	if pb.IsOff {
		return
	}

	switch pb.Type {
	case "single":
		pb.ShowIndividualProgress()
	case "total":
		pb.ShowTotalProgress()
	}
}

// Show for invididual
func (pb *ProgressBar) ShowIndividualProgress() {
	fillSize := pb.OngoingFileTransferredSize / pb.OngoingFileBlip
	fill_container := ""

	for i := 0; i < pb.BarLength; i++ {
		if i < fillSize {
			fill_container += "#"
		} else {
			fill_container += "-"
		}
	}

	if pb.RgbOn {
		fill_container = ColourSprintf(fill_container, RandomString(pb.Colours), false)
	}

	if !pb.TransferStarted {
		fmt.Printf("\n%s\n%.2f/%.2f %s %s\n", fill_container, float64(pb.OngoingFileTransferredSize)/pb.SizeConvDiv, float64(pb.OngoingFileSize)/pb.SizeConvDiv, pb.SizeUnit, pb.TrailingText)
		pb.TransferStarted = true

	} else {
		fmt.Printf("\033[F\033[F%s\n%.2f/%.2f %s %s\n", fill_container, float64(pb.OngoingFileTransferredSize)/pb.SizeConvDiv, float64(pb.OngoingFileSize)/pb.SizeConvDiv, pb.SizeUnit, pb.TrailingText)
	}
}

func (pb *ProgressBar) ShowTotalProgress() {
	fillSize := pb.TotalTransferredSize / pb.TotalTransferBlip
	fill_container := ""

	for i := 0; i < pb.BarLength; i++ {
		if i < fillSize {
			fill_container += "#"
		} else {
			fill_container += "-"
		}
	}

	if pb.RgbOn {
		fill_container = ColourSprintf(fill_container, RandomString(pb.Colours), false)
	}

	if !pb.AllTransferStarted {
		fmt.Printf("\n%s\n%.2f/%.2f %s %s\n", fill_container, float64(pb.TotalTransferredSize)/pb.SizeConvDiv, float64(pb.TotalTransferSize)/pb.SizeConvDiv, pb.SizeUnit, pb.TrailingText)
		pb.AllTransferStarted = true

	} else {
		fmt.Printf("\033[F\033[F%s\n%.2f/%.2f %s %s\n", fill_container, float64(pb.TotalTransferredSize)/pb.SizeConvDiv, float64(pb.TotalTransferSize)/pb.SizeConvDiv, pb.SizeUnit, pb.TrailingText)

	}
}

func RandomString(arr []string) string {
	if len(arr) == 0 {
		return ""
	}
	return arr[rand.Intn(len(arr))]
}
