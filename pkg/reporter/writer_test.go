package reporter

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_outputResultsToFile(t *testing.T) {
	deleteFiles(t)

	bytes, err := os.ReadFile("../../test/output/example/writer_example.txt")
	require.NoError(t, err)

	type args struct {
		results     []byte
		outputDest  string
		defaultName string
		extension   string
	}
	type want struct {
		filePath string
		data     []byte
		err      assert.ErrorAssertionFunc
	}

	tests := map[string]struct {
		args args
		want want
	}{
		"normal/existing dir": {
			args: args{
				results:     []byte("this is an example file.\nthis is for outputResultsToFile function.\n"),
				outputDest:  "../../test/output",
				defaultName: "default",
				extension:   "txt",
			},
			want: want{
				filePath: "../../test/output/default.txt",
				data:     bytes,
				err:      assert.NoError,
			},
		},
		"normal/file name is provided to outputDest": {
			args: args{
				results:     []byte("this is an example file.\nthis is for outputResultsToFile function.\n"),
				outputDest:  "../../test/output/validator_result.json",
				defaultName: "default",
				extension:   "json",
			},
			want: want{
				filePath: "../../test/output/validator_result.json",
				data:     bytes,
				err:      assert.NoError,
			},
		},
		"normal/existing dir without extension": {
			args: args{
				results:     []byte("this is an example file.\nthis is for outputResultsToFile function.\n"),
				outputDest:  "../../test/output",
				defaultName: "default",
				extension:   "",
			},
			want: want{
				filePath: "../../test/output/default",
				data:     bytes,
				err:      assert.NoError,
			},
		},
		"abnormal/empty string outputDest": {
			args: args{
				results:     []byte("this is an example file.\nthis is for outputResultsToFile function.\n"),
				outputDest:  "",
				defaultName: "default",
				extension:   ".txt",
			},
			want: want{
				data: nil,
				err:  assertRegexpError("outputDest is an empty string: "),
			},
		},
		"abnormal/non-existing dir": {
			args: args{
				results:     []byte("this is an example file.\nthis is for outputResultsToFile function.\n"),
				outputDest:  "../../test/wrong/output",
				defaultName: "result",
				extension:   "",
			},
			want: want{
				data: nil,
				err:  assertRegexpError("failed to create a file: "),
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := outputBytesToFile(tt.args.outputDest,
				tt.args.defaultName, tt.args.extension, tt.args.results)
			tt.want.err(t, err)
			if tt.want.data != nil {
				bytes, err := os.ReadFile(tt.want.filePath)
				require.NoError(t, err)
				assert.Equal(t, tt.want.data, bytes)
				err = os.Remove(tt.want.filePath)
				require.NoError(t, err)
			}
		},
		)
	}
}
