package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"
)

const uploadRoot = "./uploads"
const finalRoot = "./final"

const chunkPrefix = "chunk_"

const ENVKEY_UPLOADCHUNKER_BINDPORT = "UPLOADCHUNKER_BINDPORT"
const ENVKEY_UPLOADCHUNKER_PASSPHRASE = "UPLOADCHUNKER_PASSPHRASE"

var passphrase string

// Protocol for uploader:
// - generate uploader uuid
// - for file:
//   - generate file uuid
//   - for chunk:
//     - upload chunk (/upload-chunk)
//   - finalise file (/finalize-file)

func getUuid(w http.ResponseWriter, r *http.Request) {
	u := uuid.New()
	w.Write([]byte(u.String()))
}

func uploadChunk(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(0)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	password := r.FormValue("passphrase")
	if password != passphrase {
		http.Error(w, "invalid passphrase", 401)
		return
	}

	uploadUuid := r.FormValue("upload_id")
	fileUuid := r.FormValue("file_id")
	chunkNumber := r.FormValue("chunk_number")

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	defer file.Close()

	chunkRoot := filepath.Join(uploadRoot, uploadUuid, fileUuid)
	os.MkdirAll(chunkRoot, 0750)

	chunkPath := filepath.Join(chunkRoot, chunkPrefix+chunkNumber)

	out, err := os.Create(chunkPath)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	fmt.Printf("write chunk %s of file %s to %s\n", chunkNumber, fileUuid, chunkPath)

	w.Write([]byte("ok"))
}

func finalizeFile(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(4096)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	password := r.FormValue("passphrase")
	if password != passphrase {
		http.Error(w, "invalid passphrase", 401)
		return
	}

	uploadUuid := r.FormValue("upload_id")
	fileUuid := r.FormValue("file_id")
	filename := r.FormValue("filename")

	destPath := filepath.Join(finalRoot, fileUuid+"_"+filename)
	chunksDir := filepath.Join(uploadRoot, uploadUuid, fileUuid)

	out, err := os.Create(destPath)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer out.Close()

	for i := 0; ; i++ {
		fmt.Printf("concatenate chunk %d to %s\n", i, destPath)

		chunkPath := filepath.Join(chunksDir, chunkPrefix+strconv.Itoa(i))
		if _, err := os.Stat(chunkPath); os.IsNotExist(err) {
			break
		}

		in, err := os.Open(chunkPath)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		_, err = io.Copy(out, in)
		in.Close()

		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		os.Remove(chunkPath)
	}

	fmt.Printf("finalize file %s (%s)\n", fileUuid, filename)

	os.Remove(chunksDir)

	w.Write([]byte("ok"))
}

func index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "invalid method", 413)
		return
	}

	w.Header().Add("Content-Type", "text/html")
	w.Write([]byte(`
<!DOCTYPE html>
<html>
	<head>
		<title>Upload Chunker</title
	</head>
	<body>
		<input type="password" id="password-field" placeholder="Passwort" /> <br>
		<input type="file" id="files-field" multiple /> <br>
		<button type="button" onclick="upload()">Hochladen</button> <br> <br>
		<div id="status-field">idle</div>
	</body>
	<script>
		const statusField = document.getElementById("status-field");

		async function uploadFile(uploadid, f, passphrase) {
			const fileid = crypto.randomUUID();
			const chunkSize = 50 * 1024 * 1024;
			const totalChunks = Math.ceil(f.size / chunkSize);

			for (let i = 0; i < totalChunks; i++) {
				let ih = i + 1;
				statusField.innerHTML = f.name + ": uploading chunk " + ih.toString() + " of " + totalChunks.toString();

				const start = i * chunkSize;
				const end = Math.min(start + chunkSize, f.size);
				const chunk = f.slice(start, end);
				
				const formData = new FormData();
				formData.append("passphrase", passphrase);
				formData.append("upload_id", uploadid);
				formData.append("file_id", fileid);
				formData.append("chunk_number", i);
				formData.append("file", chunk);

				await fetch("/upload-chunk", {
					method: "POST",
					body: formData
				});
			}

			const formData = new FormData();
			formData.append("passphrase", passphrase);
			formData.append("upload_id", uploadid);
			formData.append("file_id", fileid);
			formData.append("filename", f.name);

			await fetch("/finalize-file", {
				method: "POST",
				body: formData
			});
		}
		
		async function upload() {
			const uploadid = crypto.randomUUID();
			const files = document.getElementById("files-field").files;
			const passphrase = document.getElementById("password-field").value;
			for (let i = 0; i < files.length; i++) {
				await uploadFile(uploadid, files[i], passphrase);
			}

			alert("upload complete");
			statusField.innerHTML = "idle";
		}
	</script>
</html>
	`))
}

func main() {
	os.MkdirAll(uploadRoot, 0750)
	os.MkdirAll(finalRoot, 0750)

	http.HandleFunc("/get-uuid", getUuid)
	http.HandleFunc("/upload-chunk", uploadChunk)
	http.HandleFunc("/finalize-file", finalizeFile)
	http.HandleFunc("/", index)

	bindport := os.Getenv(ENVKEY_UPLOADCHUNKER_BINDPORT)
	if bindport == "" {
		bindport = "9980"
	}

	passphrase = os.Getenv(ENVKEY_UPLOADCHUNKER_PASSPHRASE)
	if passphrase == "" {
		fmt.Println("must set passphrase")
		os.Exit(1)
	}

	fmt.Printf("Listening on :%s\n", bindport)
	http.ListenAndServe(":"+bindport, nil)
}
