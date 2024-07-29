package pdfdateparser

import (
	"reflect"
	"testing"
	"time"
)

func TestPdfDateToTime(t *testing.T) {

	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic(err)
	}
	type args struct {
		pdfdate string
	}
	tests := []struct {
		name    string
		args    args
		want    time.Time
		wantErr bool
	}{
		{name: "fq_tz",
			args: args{pdfdate: "D:20240419110302+02'00'"},
			//				  D:2024 04 19 11 03 02 +02'00'"
			//				  2024-04-19T11:03:02+02:00
			want:    time.Date(2024, 4, 19, 11, 3, 2, 0, berlin),
			wantErr: false,
		},
		{name: "z_tz",
			args: args{pdfdate: "D:20240419110302Z"},
			//				  D:2024 04 19 11 03 02 +02'00'"
			//				  2024-04-19T11:03:02+02:00
			want:    time.Date(2024, 4, 19, 11, 3, 2, 0, time.UTC),
			wantErr: false,
		},
		{name: "no_tz",
			args: args{pdfdate: "D:20240419110302"},
			//				  D:2024 04 19 11 03 02 +02'00'"
			//				  2024-04-19T11:03:02+02:00
			want:    time.Date(2024, 4, 19, 11, 3, 2, 0, time.UTC),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PdfDateToTime(tt.args.pdfdate)
			if (err != nil) != tt.wantErr {
				t.Errorf("PdfDateToTime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			t.Logf("got: %#v, err=%#v", got, err)
			t.Logf("want: %#v, err=%#v", tt.want, err)

			// Normalize to UTC. The parsed PDF Time has no named TZ, just an offset
			// DeepEqual would find this to be a diversion
			if !reflect.DeepEqual(got.UTC(), tt.want.UTC()) {
				t.Errorf("PdfDateToTime() = %v, want %v", got, tt.want)
			}
		})
	}
}
