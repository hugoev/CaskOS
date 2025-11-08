document.addEventListener('DOMContentLoaded', function() {
    const form = document.getElementById('uploadForm');
    const fileInput = document.getElementById('fileInput');
    const uploadButton = document.getElementById('uploadButton');
    const resultSection = document.getElementById('uploadResult');
    const fileLabel = document.querySelector('.file-label-text');

    // Update file label when file is selected
    fileInput.addEventListener('change', function(e) {
        const file = e.target.files[0];
        if (file) {
            fileLabel.textContent = file.name + ' (' + formatFileSize(file.size) + ')';
        } else {
            fileLabel.textContent = 'Choose a file...';
        }
    });

    // Handle form submission
    form.addEventListener('submit', async function(e) {
        e.preventDefault();

        const file = fileInput.files[0];
        if (!file) {
            showResult('error', 'Please select a file to upload.');
            return;
        }

        // Disable button and show loading state
        uploadButton.disabled = true;
        uploadButton.innerHTML = '<span class="loading"></span> Uploading...';
        resultSection.classList.add('hidden');

        const formData = new FormData();
        formData.append('file', file);

        try {
            const response = await fetch('/upload', {
                method: 'POST',
                body: formData
            });

            const data = await response.json();

            if (response.ok) {
                showSuccessResult(data, file.name);
            } else {
                showResult('error', 'Upload failed: ' + (data.error || response.statusText));
            }
        } catch (error) {
            showResult('error', 'Upload failed: ' + error.message);
        } finally {
            // Re-enable button
            uploadButton.disabled = false;
            uploadButton.innerHTML = '<span>Upload</span>';
        }
    });

    function showSuccessResult(data, fileName) {
        const objectID = data.id;
        const size = formatFileSize(data.size);
        const contentType = data.content_type || 'unknown';
        const createdAt = new Date(data.created_at).toLocaleString();
        const replicas = data.replicas || [];

        const html = `
            <h3>✅ Upload Successful!</h3>
            <p><strong>File:</strong> ${escapeHtml(fileName)}</p>
            <p><strong>Size:</strong> ${size}</p>
            <p><strong>Content Type:</strong> ${escapeHtml(contentType)}</p>
            <p><strong>Uploaded:</strong> ${createdAt}</p>
            <p><strong>Replicas:</strong> ${replicas.join(', ') || 'N/A'}</p>
            <p><strong>Object ID:</strong></p>
            <code id="objectId">${escapeHtml(objectID)}</code>
            <button class="copy-button" onclick="copyObjectId()">Copy Object ID</button>
            <p style="margin-top: 15px;">
                <strong>Download:</strong> 
                <a href="/object/${objectID}" target="_blank" style="color: #667eea;">/object/${objectID}</a>
            </p>
            <p>
                <strong>Metadata:</strong> 
                <a href="/metadata/${objectID}" target="_blank" style="color: #667eea;">/metadata/${objectID}</a>
            </p>
        `;

        resultSection.className = 'result-section success';
        resultSection.innerHTML = html;
        resultSection.classList.remove('hidden');

        // Store object ID globally for copy function
        window.currentObjectId = objectID;
    }

    function showResult(type, message) {
        const className = type === 'error' ? 'result-section error' : 'result-section success';
        resultSection.className = className;
        resultSection.innerHTML = `<h3>${type === 'error' ? '❌ Error' : '✅ Success'}</h3><p>${escapeHtml(message)}</p>`;
        resultSection.classList.remove('hidden');
    }

    function formatFileSize(bytes) {
        if (bytes === 0) return '0 Bytes';
        const k = 1024;
        const sizes = ['Bytes', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
    }

    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
});

// Copy object ID to clipboard
function copyObjectId() {
    const objectId = window.currentObjectId;
    if (!objectId) return;

    navigator.clipboard.writeText(objectId).then(function() {
        const button = event.target;
        const originalText = button.textContent;
        button.textContent = 'Copied!';
        button.style.background = '#28a745';
        
        setTimeout(function() {
            button.textContent = originalText;
            button.style.background = '#667eea';
        }, 2000);
    }).catch(function(err) {
        console.error('Failed to copy:', err);
        alert('Failed to copy to clipboard');
    });
}

