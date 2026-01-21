package server

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"

	"github.com/linus/recal/internal/feeds"
)

// SlugManage serves the user-facing feed management page
func (s *Server) SlugManage(w http.ResponseWriter, r *http.Request) {
	// Record request metrics
	s.requestMetrics.RecordRequest()

	// Handle both GET (show page) and PUT (update feed)
	switch r.Method {
	case http.MethodGet:
		s.showManagePage(w, r)
	case http.MethodPut:
		s.updateFeedFromManagePage(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// showManagePage displays the user management interface
func (s *Server) showManagePage(w http.ResponseWriter, r *http.Request) {
	preferSwedish := prefersSwedish(r)

	// Extract slug from path
	slug := extractSlugFromPath(r.URL.Path, "/feed/")
	if slug == "" {
		http.Error(w, "Invalid slug", http.StatusBadRequest)
		return
	}

	// Get feed from manager
	feed, err := s.feedManager.Get(slug)
	if err != nil {
		if err == feeds.ErrFeedNotFound || err == feeds.ErrInvalidUUID {
			http.Error(w, "Feed not found", http.StatusNotFound)
			return
		}
		log.Printf("Error getting feed %s: %v", slug, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	templateSource := userManagePageTemplateEN
	if preferSwedish {
		templateSource = userManagePageTemplateSV
	}

	// Parse template
	tmpl, err := template.New("manage").Parse(templateSource)
	if err != nil {
		http.Error(w, "Failed to parse template", http.StatusInternalServerError)
		log.Printf("Manage page template parse error: %v", err)
		return
	}

	data := struct {
		BaseURL     string
		Slug        string
		Description string
		Filters     map[string]string
		Owner       string
	}{
		BaseURL:     s.cfg.Server.BaseURL,
		Slug:        feed.Slug,
		Description: feed.Description,
		Filters:     feed.Filters,
		Owner:       feed.Owner,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		log.Printf("Manage page template execute error: %v", err)
		return
	}
}

// updateFeedFromManagePage handles PUT requests to update feed filters
func (s *Server) updateFeedFromManagePage(w http.ResponseWriter, r *http.Request) {
	// Extract slug from path
	slug := extractSlugFromPath(r.URL.Path, "/feed/")
	if slug == "" {
		http.Error(w, "Invalid slug", http.StatusBadRequest)
		return
	}

	// Verify feed exists
	_, err := s.feedManager.Get(slug)
	if err != nil {
		if err == feeds.ErrFeedNotFound || err == feeds.ErrInvalidUUID {
			http.Error(w, "Feed not found", http.StatusNotFound)
			return
		}
		log.Printf("Error getting feed %s: %v", slug, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Parse request body
	var req struct {
		Description string            `json:"description"`
		Filters     map[string]string `json:"filters"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate filters
	if len(req.Filters) == 0 {
		http.Error(w, "At least one filter is required", http.StatusBadRequest)
		return
	}

	// Update the feed using manager's Update method
	_, err = s.feedManager.Update(slug, req.Description, req.Filters)
	if err != nil {
		log.Printf("Error updating feed %s: %v", slug, err)
		http.Error(w, "Failed to update feed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"message":"Feed updated successfully"}`)); err != nil {
		log.Printf("Failed to write feed update response: %v", err)
	}
}

const userManagePageTemplateEN = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Manage Your Feed - ReCal</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px;
            display: flex;
            align-items: center;
            justify-content: center;
        }

        .container {
            background: white;
            max-width: 900px;
            width: 100%;
            border-radius: 12px;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
            padding: 40px;
        }

        h1 {
            color: #333;
            font-size: 28px;
            margin-bottom: 10px;
        }

        .subtitle {
            color: #666;
            font-size: 14px;
            margin-bottom: 30px;
        }

        .feed-url {
            background: #f8f9fa;
            padding: 15px;
            border-radius: 8px;
            margin-bottom: 30px;
            border: 2px solid #e9ecef;
        }

        .feed-url-label {
            font-size: 12px;
            color: #666;
            font-weight: 600;
            margin-bottom: 5px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }

        .feed-url-value {
            font-family: monospace;
            font-size: 13px;
            color: #495057;
            word-break: break-all;
            display: flex;
            align-items: center;
            gap: 10px;
        }

        .copy-btn {
            background: #667eea;
            color: white;
            border: none;
            padding: 6px 12px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 12px;
            white-space: nowrap;
            transition: background 0.2s;
        }

        .copy-btn:hover {
            background: #5568d3;
        }

        .calendar-apps {
            margin-top: 15px;
            padding-top: 15px;
            border-top: 1px solid #e9ecef;
        }

        .calendar-apps-label {
            font-size: 12px;
            color: #666;
            font-weight: 600;
            margin-bottom: 10px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }

        .calendar-apps-buttons {
            display: flex;
            flex-wrap: wrap;
            gap: 8px;
        }

        .calendar-apps-buttons button {
            background: #28a745;
            color: white;
            border: none;
            padding: 8px 14px;
            border-radius: 6px;
            cursor: pointer;
            font-size: 13px;
            transition: all 0.2s;
            display: inline-flex;
            align-items: center;
            gap: 6px;
        }

        .calendar-apps-buttons button:hover {
            background: #218838;
            transform: translateY(-1px);
        }

        .form-group {
            margin-bottom: 25px;
        }

        label {
            display: block;
            margin-bottom: 8px;
            font-weight: 600;
            color: #333;
            font-size: 14px;
        }

        input[type="text"],
        select {
            width: 100%;
            padding: 10px;
            border: 2px solid #e1e4e8;
            border-radius: 6px;
            font-size: 14px;
            transition: border-color 0.2s;
        }

        input[type="text"]:focus,
        select:focus {
            outline: none;
            border-color: #667eea;
        }

        input[readonly] {
            background: #f5f5f5;
            cursor: not-allowed;
        }

        .info-box {
            background: #e8f4f8;
            padding: 15px;
            border-radius: 6px;
            margin-bottom: 20px;
            font-size: 13px;
            color: #0c5460;
            border-left: 4px solid #667eea;
        }

        .info-box strong {
            display: block;
            margin-bottom: 5px;
        }

        .info-box ul {
            margin: 8px 0 0 20px;
            padding: 0;
        }

        .checkbox-group {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
            gap: 10px;
            padding: 15px;
            background: #f9f9f9;
            border-radius: 6px;
            max-height: 200px;
            overflow-y: auto;
        }

        .checkbox-label {
            display: flex;
            align-items: center;
            gap: 8px;
            cursor: pointer;
            font-size: 13px;
            line-height: 1.4;
            padding: 2px 0;
        }

        .checkbox-label input[type="checkbox"] {
            margin: 0;
            flex-shrink: 0;
            width: 16px;
            height: 16px;
            cursor: pointer;
        }

        .special-filters {
            padding: 15px;
            background: #f9f9f9;
            border-radius: 6px;
        }

        .special-filters .checkbox-label {
            margin-bottom: 10px;
        }

        .btn-group {
            display: flex;
            gap: 10px;
            margin-top: 30px;
            padding-top: 20px;
            border-top: 2px solid #e9ecef;
            flex-wrap: wrap;
        }

        .btn {
            padding: 12px 24px;
            border: none;
            border-radius: 6px;
            font-size: 15px;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.2s;
        }

        .btn-primary {
            background: #667eea;
            color: white;
            flex: 1;
        }

        .btn-primary:hover {
            background: #5568d3;
            transform: translateY(-1px);
            box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
        }

        .btn-secondary {
            background: #6c757d;
            color: white;
        }

        .btn-secondary:hover {
            background: #5a6268;
        }

        .btn-preview {
            background: white;
            color: #667eea;
            border: 2px solid #667eea;
        }

        .btn-preview:hover {
            background: #f8f9ff;
        }

        .message {
            padding: 12px;
            border-radius: 6px;
            margin-bottom: 20px;
            display: none;
        }

        .message.success {
            background: #d4edda;
            color: #155724;
            border: 1px solid #c3e6cb;
            display: block;
        }

        .message.error {
            background: #f8d7da;
            color: #721c24;
            border: 1px solid #f5c6cb;
            display: block;
        }

        details {
            margin-top: 15px;
        }

        summary {
            cursor: pointer;
            color: #667eea;
            font-weight: 600;
            font-size: 14px;
            padding: 10px;
            background: #f8f9ff;
            border-radius: 6px;
        }

        summary:hover {
            background: #eef1ff;
        }

        .advanced-content {
            margin-top: 15px;
            padding: 15px;
            background: #f9f9f9;
            border-radius: 6px;
        }

        @media (max-width: 768px) {
            .container {
                padding: 15px;
            }

            .btn-group {
                flex-direction: column;
            }

            .btn {
                width: 100%;
            }

            .feed-url-value {
                flex-direction: column;
                gap: 10px;
            }

            .copy-btn {
                width: 100%;
            }

            select {
                font-size: 16px; /* Prevents zoom on iOS */
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Manage Your Calendar Feed</h1>
        <p class="subtitle">Update your filter settings to customize which events appear in your calendar</p>

        <div class="feed-url">
            <div class="feed-url-label">Your Feed URL</div>
            <div class="feed-url-value">
                <span id="feedUrl">{{.BaseURL}}/feed/{{.Slug}}</span>
                <button class="copy-btn" onclick="copyFeedUrl()">Copy</button>
            </div>
            <div class="calendar-apps">
                <div class="calendar-apps-label">Add to your calendar app</div>
                <div id="calendarApps" class="calendar-apps-buttons"></div>
            </div>
            {{if .Owner}}<div style="margin-top: 10px; font-size: 13px; color: #666;"><span style="font-weight: 600;">Owner:</span> {{.Owner}}</div>{{end}}
        </div>

        <div id="message" class="message"></div>

        <form id="manageForm">
            <div class="form-group">
                <label>Feed Name</label>
                <input type="text" id="feedDescription" value="{{.Description}}" maxlength="500" placeholder="e.g., My Grade 4 Stockholm Calendar">
                <small style="color: #666; display: block; margin-top: 5px;">This name is for your reference only</small>
            </div>

            <div class="form-group">
                <label>Filters</label>
                <div class="info-box">
                    <strong>How filters work:</strong>
                    Events matching these filters will be <strong>removed</strong> from your calendar.
                    <ul>
                        <li><code>Grade 4</code> removes Grade 5+ events (keeps grades 1-4)</li>
                        <li><code>Lodge: Göta</code> checked keeps Göta events (unchecked lodges are removed)</li>
                    </ul>
                </div>

                <div style="margin-bottom: 20px;">
                    <label>Grade Filter (Remove events above this grade)</label>
                    <select id="gradFilter">
                        <option value="">No grade filter</option>
                        <option value="1">Grade 1 (remove 2-12)</option>
                        <option value="2">Grade 2 (remove 3-12)</option>
                        <option value="3">Grade 3 (remove 4-12)</option>
                        <option value="4">Grade 4 (remove 5-12)</option>
                        <option value="5">Grade 5 (remove 6-12)</option>
                        <option value="6">Grade 6 (remove 7-12)</option>
                        <option value="7">Grade 7 (remove 8-12)</option>
                        <option value="8">Grade 8 (remove 9-12)</option>
                        <option value="9">Grade 9 (remove 10-12)</option>
                        <option value="10">Grade 10 (remove 11-12)</option>
                        <option value="11">Grade 11 (remove 12)</option>
                        <option value="12">Grade 12 (remove none)</option>
                    </select>
                </div>

                <div style="margin-bottom: 20px;">
                    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;">
                        <label style="margin-bottom: 0;">Lodge Filter (Keep events from these lodges)</label>
                        <div style="display: flex; gap: 8px;">
                            <button type="button" onclick="selectAllLodges()" style="background: #667eea; color: white; border: none; padding: 4px 10px; border-radius: 4px; font-size: 12px; cursor: pointer;">Select All</button>
                            <button type="button" onclick="deselectAllLodges()" style="background: #6c757d; color: white; border: none; padding: 4px 10px; border-radius: 4px; font-size: 12px; cursor: pointer;">Deselect All</button>
                        </div>
                    </div>
                    <div id="lodgeCheckboxes" class="checkbox-group"></div>
                    <small style="color: #666; display: block; margin-top: 5px;">Events not matching any lodge are always kept</small>
                </div>

                <div style="margin-bottom: 20px;">
                    <label>Special Filters</label>
                    <div class="special-filters">
                        <label class="checkbox-label">
                            <input type="checkbox" id="removeUnconfirmed">
                            <span>Remove unconfirmed events</span>
                        </label>
                        <label class="checkbox-label">
                            <input type="checkbox" id="removeInstallt">
                            <span>Remove cancelled events (INSTÄLLT)</span>
                        </label>
                    </div>
                </div>
            </div>

            <div class="btn-group">
                <button type="button" class="btn btn-secondary" onclick="resetForm()" style="background: #dc3545;">Cancel</button>
                <button type="button" class="btn btn-secondary" onclick="previewFeed()">Preview Calendar</button>
                <button type="submit" class="btn btn-primary">Save Changes</button>
            </div>
        </form>
    </div>

    <script>
        const slug = '{{.Slug}}';
        const feedUrl = '{{.BaseURL}}/feed/{{.Slug}}';
        let availableLodges = [];
        let originalFilters = {};
        let originalDescription = '';

        // Load page
        document.addEventListener('DOMContentLoaded', () => {
            loadLodges();
            loadCurrentFilters();
            setupCalendarApps();
        });

        // Setup calendar app buttons
        function setupCalendarApps() {
            const container = document.getElementById('calendarApps');

            // Detect platform
            const ua = navigator.userAgent;
            const isMac = /Mac/.test(ua) && !/iPhone|iPad/.test(ua);
            const isIOS = /iPhone|iPad|iPod/.test(ua);

            // Get URLs
            const webcalUrl = feedUrl.replace(/^https?:\/\//, 'webcal://');
            const httpsUrl = encodeURIComponent(feedUrl);

            // Apple Calendar (macOS/iOS)
            if (isMac || isIOS) {
                const btn = document.createElement('button');
                btn.innerHTML = '📅 Apple Calendar';
                btn.onclick = () => { window.location.href = webcalUrl; };
                container.appendChild(btn);
            }

            // Google Calendar
            const googleBtn = document.createElement('button');
            googleBtn.innerHTML = '🌐 Google Calendar';
            googleBtn.onclick = () => {
                window.open('https://calendar.google.com/calendar/render?cid=' + httpsUrl, '_blank');
            };
            container.appendChild(googleBtn);

            // Outlook.com
            const outlookBtn = document.createElement('button');
            outlookBtn.innerHTML = '📧 Outlook.com';
            outlookBtn.onclick = () => {
                window.open('https://outlook.live.com/calendar/0/addfromweb?url=' + httpsUrl, '_blank');
            };
            container.appendChild(outlookBtn);

            // Generic webcal
            const webcalBtn = document.createElement('button');
            webcalBtn.innerHTML = '📱 Other app';
            webcalBtn.onclick = () => {
                navigator.clipboard.writeText(webcalUrl).then(() => {
                    showMessage('webcal:// URL copied! Paste into your calendar app.', 'success');
                }).catch(() => {
                    prompt('Copy this URL into your calendar app:', webcalUrl);
                });
            };
            container.appendChild(webcalBtn);
        }

        // Load available lodges
        async function loadLodges() {
            try {
                const response = await fetch('/api/lodges');
                if (!response.ok) return;
                const data = await response.json();
                availableLodges = data.lodges || [];
                populateLodgeCheckboxes();
            } catch (error) {
                console.error('Failed to load lodges:', error);
            }
        }

        // Load current filters from template data
        function loadCurrentFilters() {
            const filters = {{.Filters}};

            // Store original values for reset
            originalFilters = JSON.parse(JSON.stringify(filters));
            originalDescription = '{{.Description}}';

            // Set grade
            if (filters.Grad) {
                document.getElementById('gradFilter').value = filters.Grad;
            }

            // Set lodges (will be set after lodges are loaded)
            if (filters.Loge) {
                const selectedLodges = filters.Loge.split(',').map(l => l.trim());
                window.selectedLodgesInitial = selectedLodges;
            }

            // Set special filters
            document.getElementById('removeUnconfirmed').checked = filters.RemoveUnconfirmed === 'true';
            document.getElementById('removeInstallt').checked = filters.RemoveInstallt === 'true';
        }

        // Populate lodge checkboxes
        // Saved Loge filter contains EXCLUDED lodges, so those should be UNchecked
        // All other lodges should be checked (included by default)
        function populateLodgeCheckboxes() {
            const container = document.getElementById('lodgeCheckboxes');
            const excluded = window.selectedLodgesInitial || [];

            container.innerHTML = availableLodges.map(lodge => {
                // Checked = included (kept), Unchecked = excluded (removed)
                const checked = excluded.includes(lodge) ? '' : 'checked';
                return '<label class="checkbox-label">' +
                    '<input type="checkbox" value="' + escapeHtml(lodge) + '" ' + checked + '>' +
                    '<span>' + escapeHtml(lodge) + '</span>' +
                    '</label>';
            }).join('');
        }

        // Copy feed URL
        function copyFeedUrl() {
            const url = document.getElementById('feedUrl').textContent;
            navigator.clipboard.writeText(url).then(() => {
                showMessage('Feed URL copied to clipboard!', 'success');
            }).catch(() => {
                showMessage('Failed to copy URL', 'error');
            });
        }

        // Reset form to original values
        function resetForm() {
            // Reset description
            document.getElementById('feedDescription').value = originalDescription;

            // Reset grade
            document.getElementById('gradFilter').value = originalFilters.Grad || '';

            // Reset lodges - saved Loge contains EXCLUDED lodges, so invert the check
            const excludedLodges = originalFilters.Loge ? originalFilters.Loge.split(',').map(l => l.trim()) : [];
            document.querySelectorAll('#lodgeCheckboxes input[type="checkbox"]').forEach(cb => {
                cb.checked = !excludedLodges.includes(cb.value);
            });

            // Reset special filters
            document.getElementById('removeUnconfirmed').checked = originalFilters.RemoveUnconfirmed === 'true';
            document.getElementById('removeInstallt').checked = originalFilters.RemoveInstallt === 'true';

            // Show message
            showMessage('Form reset to saved values', 'success');
        }

        // Preview feed
        function previewFeed() {
            const filters = collectFilters();
            const queryString = buildQueryString(filters);
            window.open('/feed/' + slug + '/preview?' + queryString, '_blank');
        }

        // Submit form
        document.getElementById('manageForm').addEventListener('submit', async (e) => {
            e.preventDefault();

            const description = document.getElementById('feedDescription').value.trim();
            const filters = collectFilters();

            if (Object.keys(filters).length === 0) {
                showMessage('Please select at least one filter', 'error');
                return;
            }

            try {
                const response = await fetch('/feed/' + slug + '/edit', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ description, filters })
                });

                if (!response.ok) {
                    const error = await response.text();
                    throw new Error(error);
                }

                showMessage('Feed updated successfully! Your calendar will reflect the changes.', 'success');
            } catch (error) {
                showMessage('Failed to update feed: ' + error.message, 'error');
            }
        });

        // Collect filters from form
        function collectFilters() {
            const filters = {};

            // Grade filter
            const grad = document.getElementById('gradFilter').value;
            if (grad) filters.Grad = grad;

            // Lodge filter - send UNCHECKED lodges (the ones to exclude/remove)
            const uncheckedLodges = Array.from(document.querySelectorAll('#lodgeCheckboxes input:not(:checked)'))
                .map(cb => cb.value);
            if (uncheckedLodges.length > 0) filters.Loge = uncheckedLodges.join(',');

            // Special filters
            if (document.getElementById('removeUnconfirmed').checked) {
                filters.RemoveUnconfirmed = 'true';
            }
            if (document.getElementById('removeInstallt').checked) {
                filters.RemoveInstallt = 'true';
            }

            return filters;
        }

        // Build query string
        function buildQueryString(filters) {
            return Object.entries(filters)
                .map(([key, value]) => encodeURIComponent(key) + '=' + encodeURIComponent(value))
                .join('&');
        }

        // Show message
        function showMessage(text, type) {
            const msg = document.getElementById('message');
            msg.textContent = text;
            msg.className = 'message ' + type;
            setTimeout(() => {
                msg.className = 'message';
            }, 5000);
        }

        // Select all lodges
        function selectAllLodges() {
            document.querySelectorAll('#lodgeCheckboxes input[type="checkbox"]').forEach(cb => {
                cb.checked = true;
            });
        }

        // Deselect all lodges
        function deselectAllLodges() {
            document.querySelectorAll('#lodgeCheckboxes input[type="checkbox"]').forEach(cb => {
                cb.checked = false;
            });
        }

        // Escape HTML
        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }
    </script>
</body>
</html>
`

const userManagePageTemplateSV = `<!DOCTYPE html>
<html lang="sv">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Hantera din feed - ReCal</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px;
            display: flex;
            align-items: center;
            justify-content: center;
        }

        .container {
            background: white;
            max-width: 900px;
            width: 100%;
            border-radius: 12px;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
            padding: 40px;
        }

        h1 {
            color: #333;
            font-size: 28px;
            margin-bottom: 10px;
        }

        .subtitle {
            color: #666;
            font-size: 14px;
            margin-bottom: 30px;
        }

        .feed-url {
            background: #f8f9fa;
            padding: 15px;
            border-radius: 8px;
            margin-bottom: 30px;
            border: 2px solid #e9ecef;
        }

        .feed-url-label {
            font-size: 12px;
            color: #666;
            font-weight: 600;
            margin-bottom: 5px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }

        .feed-url-value {
            font-family: monospace;
            font-size: 13px;
            color: #495057;
            word-break: break-all;
            display: flex;
            align-items: center;
            gap: 10px;
        }

        .copy-btn {
            background: #667eea;
            color: white;
            border: none;
            padding: 6px 12px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 12px;
            white-space: nowrap;
            transition: background 0.2s;
        }

        .copy-btn:hover {
            background: #5568d3;
        }

        .calendar-apps {
            margin-top: 15px;
            padding-top: 15px;
            border-top: 1px solid #e9ecef;
        }

        .calendar-apps-label {
            font-size: 12px;
            color: #666;
            font-weight: 600;
            margin-bottom: 10px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }

        .calendar-apps-buttons {
            display: flex;
            flex-wrap: wrap;
            gap: 8px;
        }

        .calendar-apps-buttons button {
            background: #28a745;
            color: white;
            border: none;
            padding: 8px 14px;
            border-radius: 6px;
            cursor: pointer;
            font-size: 13px;
            transition: all 0.2s;
            display: inline-flex;
            align-items: center;
            gap: 6px;
        }

        .calendar-apps-buttons button:hover {
            background: #218838;
            transform: translateY(-1px);
        }

        .form-group {
            margin-bottom: 25px;
        }

        label {
            display: block;
            margin-bottom: 8px;
            font-weight: 600;
            color: #333;
            font-size: 14px;
        }

        input[type="text"],
        select {
            width: 100%;
            padding: 10px;
            border: 2px solid #e1e4e8;
            border-radius: 6px;
            font-size: 14px;
            transition: border-color 0.2s;
        }

        input[type="text"]:focus,
        select:focus {
            outline: none;
            border-color: #667eea;
        }

        input[readonly] {
            background: #f5f5f5;
            cursor: not-allowed;
        }

        .info-box {
            background: #e8f4f8;
            padding: 15px;
            border-radius: 6px;
            margin-bottom: 20px;
            font-size: 13px;
            color: #0c5460;
            border-left: 4px solid #667eea;
        }

        .info-box strong {
            display: block;
            margin-bottom: 5px;
        }

        .info-box ul {
            margin: 8px 0 0 20px;
            padding: 0;
        }

        .checkbox-group {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
            gap: 10px;
            padding: 15px;
            background: #f9f9f9;
            border-radius: 6px;
            max-height: 200px;
            overflow-y: auto;
        }

        .checkbox-label {
            display: flex;
            align-items: center;
            gap: 8px;
            cursor: pointer;
            font-size: 13px;
            line-height: 1.4;
            padding: 2px 0;
        }

        .checkbox-label input[type="checkbox"] {
            margin: 0;
            flex-shrink: 0;
            width: 16px;
            height: 16px;
            cursor: pointer;
        }

        .special-filters {
            padding: 15px;
            background: #f9f9f9;
            border-radius: 6px;
        }

        .special-filters .checkbox-label {
            margin-bottom: 10px;
        }

        .btn-group {
            display: flex;
            gap: 10px;
            margin-top: 30px;
            padding-top: 20px;
            border-top: 2px solid #e9ecef;
            flex-wrap: wrap;
        }

        .btn {
            padding: 12px 24px;
            border: none;
            border-radius: 6px;
            font-size: 15px;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.2s;
        }

        .btn-primary {
            background: #667eea;
            color: white;
            flex: 1;
        }

        .btn-primary:hover {
            background: #5568d3;
            transform: translateY(-1px);
            box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
        }

        .btn-secondary {
            background: #6c757d;
            color: white;
        }

        .btn-secondary:hover {
            background: #5a6268;
        }

        .btn-preview {
            background: white;
            color: #667eea;
            border: 2px solid #667eea;
        }

        .btn-preview:hover {
            background: #f8f9ff;
        }

        .message {
            padding: 12px;
            border-radius: 6px;
            margin-bottom: 20px;
            display: none;
        }

        .message.success {
            background: #d4edda;
            color: #155724;
            border: 1px solid #c3e6cb;
            display: block;
        }

        .message.error {
            background: #f8d7da;
            color: #721c24;
            border: 1px solid #f5c6cb;
            display: block;
        }

        details {
            margin-top: 15px;
        }

        summary {
            cursor: pointer;
            color: #667eea;
            font-weight: 600;
            font-size: 14px;
            padding: 10px;
            background: #f8f9ff;
            border-radius: 6px;
        }

        summary:hover {
            background: #eef1ff;
        }

        .advanced-content {
            margin-top: 15px;
            padding: 15px;
            background: #f9f9f9;
            border-radius: 6px;
        }

        @media (max-width: 768px) {
            .container {
                padding: 15px;
            }

            .btn-group {
                flex-direction: column;
            }

            .btn {
                width: 100%;
            }

            .feed-url-value {
                flex-direction: column;
                gap: 10px;
            }

            .copy-btn {
                width: 100%;
            }

            select {
                font-size: 16px; /* Prevents zoom on iOS */
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Hantera din kalenderfeed</h1>
        <p class="subtitle">Uppdatera dina filterinställningar för att anpassa vilka händelser som visas i din kalender</p>

        <div class="feed-url">
            <div class="feed-url-label">Din feed-URL</div>
            <div class="feed-url-value">
                <span id="feedUrl">{{.BaseURL}}/feed/{{.Slug}}</span>
                <button class="copy-btn" onclick="copyFeedUrl()">Kopiera</button>
            </div>
            <div class="calendar-apps">
                <div class="calendar-apps-label">Lägg till i din kalenderapp</div>
                <div id="calendarApps" class="calendar-apps-buttons"></div>
            </div>
            {{if .Owner}}<div style="margin-top: 10px; font-size: 13px; color: #666;"><span style="font-weight: 600;">Ägare:</span> {{.Owner}}</div>{{end}}
        </div>

        <div id="message" class="message"></div>

        <form id="manageForm">
            <div class="form-group">
                <label>Feed-namn</label>
                <input type="text" id="feedDescription" value="{{.Description}}" maxlength="500" placeholder="t.ex. Min Grad 4 Stockholm-kalender">
                <small style="color: #666; display: block; margin-top: 5px;">Detta namn är bara för din egen referens</small>
            </div>

            <div class="form-group">
                <label>Filter</label>
                <div class="info-box">
                    <strong>Så fungerar filtren:</strong>
                    Händelser som matchar dessa filter kommer att <strong>tas bort</strong> från din kalender.
                    <ul>
                        <li><code>Grad 4</code> tar bort Grad 5+ (behåller grad 1-4)</li>
                        <li><code>Loge: Göta</code> markerad behåller Göta-händelser (avmarkerade loger tas bort)</li>
                    </ul>
                </div>

                <div style="margin-bottom: 20px;">
                    <label>Gradfilter (ta bort händelser över denna grad)</label>
                    <select id="gradFilter">
                        <option value="">Inget gradfilter</option>
                        <option value="1">Grad 1 (tar bort 2-12)</option>
                        <option value="2">Grad 2 (tar bort 3-12)</option>
                        <option value="3">Grad 3 (tar bort 4-12)</option>
                        <option value="4">Grad 4 (tar bort 5-12)</option>
                        <option value="5">Grad 5 (tar bort 6-12)</option>
                        <option value="6">Grad 6 (tar bort 7-12)</option>
                        <option value="7">Grad 7 (tar bort 8-12)</option>
                        <option value="8">Grad 8 (tar bort 9-12)</option>
                        <option value="9">Grad 9 (tar bort 10-12)</option>
                        <option value="10">Grad 10 (tar bort 11-12)</option>
                        <option value="11">Grad 11 (tar bort 12)</option>
                        <option value="12">Grad 12 (tar bort inget)</option>
                    </select>
                </div>

                <div style="margin-bottom: 20px;">
                    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;">
                        <label style="margin-bottom: 0;">Logefilter (behåll händelser från dessa loger)</label>
                        <div style="display: flex; gap: 8px;">
                            <button type="button" onclick="selectAllLodges()" style="background: #667eea; color: white; border: none; padding: 4px 10px; border-radius: 4px; font-size: 12px; cursor: pointer;">Markera alla</button>
                            <button type="button" onclick="deselectAllLodges()" style="background: #6c757d; color: white; border: none; padding: 4px 10px; border-radius: 4px; font-size: 12px; cursor: pointer;">Avmarkera alla</button>
                        </div>
                    </div>
                    <div id="lodgeCheckboxes" class="checkbox-group"></div>
                    <small style="color: #666; display: block; margin-top: 5px;">Händelser som inte matchar någon loge behålls alltid</small>
                </div>

                <div style="margin-bottom: 20px;">
                    <label>Specialfilter</label>
                    <div class="special-filters">
                        <label class="checkbox-label">
                            <input type="checkbox" id="removeUnconfirmed">
                            <span>Ta bort obekräftade händelser</span>
                        </label>
                        <label class="checkbox-label">
                            <input type="checkbox" id="removeInstallt">
                            <span>Ta bort inställda händelser (INSTÄLLT)</span>
                        </label>
                    </div>
                </div>
            </div>

            <div class="btn-group">
                <button type="button" class="btn btn-secondary" onclick="resetForm()" style="background: #dc3545;">Avbryt</button>
                <button type="button" class="btn btn-secondary" onclick="previewFeed()">Förhandsgranska kalender</button>
                <button type="submit" class="btn btn-primary">Spara ändringar</button>
            </div>
        </form>
    </div>

    <script>
        const slug = '{{.Slug}}';
        const feedUrl = '{{.BaseURL}}/feed/{{.Slug}}';
        let availableLodges = [];
        let originalFilters = {};
        let originalDescription = '';

        // Load page
        document.addEventListener('DOMContentLoaded', () => {
            loadLodges();
            loadCurrentFilters();
            setupCalendarApps();
        });

        // Setup calendar app buttons
        function setupCalendarApps() {
            const container = document.getElementById('calendarApps');

            // Detect platform
            const ua = navigator.userAgent;
            const isMac = /Mac/.test(ua) && !/iPhone|iPad/.test(ua);
            const isIOS = /iPhone|iPad|iPod/.test(ua);

            // Get URLs
            const webcalUrl = feedUrl.replace(/^https?:\/\//, 'webcal://');
            const httpsUrl = encodeURIComponent(feedUrl);

            // Apple Calendar (macOS/iOS)
            if (isMac || isIOS) {
                const btn = document.createElement('button');
                btn.innerHTML = '📅 Apple Kalender';
                btn.onclick = () => { window.location.href = webcalUrl; };
                container.appendChild(btn);
            }

            // Google Calendar
            const googleBtn = document.createElement('button');
            googleBtn.innerHTML = '🌐 Google Kalender';
            googleBtn.onclick = () => {
                window.open('https://calendar.google.com/calendar/render?cid=' + httpsUrl, '_blank');
            };
            container.appendChild(googleBtn);

            // Outlook.com
            const outlookBtn = document.createElement('button');
            outlookBtn.innerHTML = '📧 Outlook.com';
            outlookBtn.onclick = () => {
                window.open('https://outlook.live.com/calendar/0/addfromweb?url=' + httpsUrl, '_blank');
            };
            container.appendChild(outlookBtn);

            // Generic webcal
            const webcalBtn = document.createElement('button');
            webcalBtn.innerHTML = '📱 Annan app';
            webcalBtn.onclick = () => {
                navigator.clipboard.writeText(webcalUrl).then(() => {
                    showMessage('webcal:// URL kopierad! Klistra in i din kalenderapp.', 'success');
                }).catch(() => {
                    prompt('Kopiera denna URL till din kalenderapp:', webcalUrl);
                });
            };
            container.appendChild(webcalBtn);
        }

        // Load available lodges
        async function loadLodges() {
            try {
                const response = await fetch('/api/lodges');
                if (!response.ok) return;
                const data = await response.json();
                availableLodges = data.lodges || [];
                populateLodgeCheckboxes();
            } catch (error) {
                console.error('Failed to load lodges:', error);
            }
        }

        // Load current filters from template data
        function loadCurrentFilters() {
            const filters = {{.Filters}};

            // Store original values for reset
            originalFilters = JSON.parse(JSON.stringify(filters));
            originalDescription = '{{.Description}}';

            // Set grade
            if (filters.Grad) {
                document.getElementById('gradFilter').value = filters.Grad;
            }

            // Set lodges (will be set after lodges are loaded)
            if (filters.Loge) {
                const selectedLodges = filters.Loge.split(',').map(l => l.trim());
                window.selectedLodgesInitial = selectedLodges;
            }

            // Set special filters
            document.getElementById('removeUnconfirmed').checked = filters.RemoveUnconfirmed === 'true';
            document.getElementById('removeInstallt').checked = filters.RemoveInstallt === 'true';
        }

        // Populate lodge checkboxes
        // Saved Loge filter contains EXCLUDED lodges, so those should be UNchecked
        // All other lodges should be checked (included by default)
        function populateLodgeCheckboxes() {
            const container = document.getElementById('lodgeCheckboxes');
            const excluded = window.selectedLodgesInitial || [];

            container.innerHTML = availableLodges.map(lodge => {
                // Checked = included (kept), Unchecked = excluded (removed)
                const checked = excluded.includes(lodge) ? '' : 'checked';
                return '<label class="checkbox-label">' +
                    '<input type="checkbox" value="' + escapeHtml(lodge) + '" ' + checked + '>' +
                    '<span>' + escapeHtml(lodge) + '</span>' +
                    '</label>';
            }).join('');
        }

        // Copy feed URL
        function copyFeedUrl() {
            const url = document.getElementById('feedUrl').textContent;
            navigator.clipboard.writeText(url).then(() => {
                showMessage('Feed-URL kopierad till urklipp!', 'success');
            }).catch(() => {
                showMessage('Kunde inte kopiera URL', 'error');
            });
        }

        // Reset form to original values
        function resetForm() {
            // Reset description
            document.getElementById('feedDescription').value = originalDescription;

            // Reset grade
            document.getElementById('gradFilter').value = originalFilters.Grad || '';

            // Reset lodges - saved Loge contains EXCLUDED lodges, so invert the check
            const excludedLodges = originalFilters.Loge ? originalFilters.Loge.split(',').map(l => l.trim()) : [];
            document.querySelectorAll('#lodgeCheckboxes input[type="checkbox"]').forEach(cb => {
                cb.checked = !excludedLodges.includes(cb.value);
            });

            // Reset special filters
            document.getElementById('removeUnconfirmed').checked = originalFilters.RemoveUnconfirmed === 'true';
            document.getElementById('removeInstallt').checked = originalFilters.RemoveInstallt === 'true';

            // Show message
            showMessage('Formuläret återställdes till sparade värden', 'success');
        }

        // Preview feed
        function previewFeed() {
            const filters = collectFilters();
            const queryString = buildQueryString(filters);
            window.open('/feed/' + slug + '/preview?' + queryString, '_blank');
        }

        // Submit form
        document.getElementById('manageForm').addEventListener('submit', async (e) => {
            e.preventDefault();

            const description = document.getElementById('feedDescription').value.trim();
            const filters = collectFilters();

            if (Object.keys(filters).length === 0) {
                showMessage('Välj minst ett filter', 'error');
                return;
            }

            try {
                const response = await fetch('/feed/' + slug + '/edit', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ description, filters })
                });

                if (!response.ok) {
                    const error = await response.text();
                    throw new Error(error);
                }

                showMessage('Feed uppdaterad! Din kalender kommer att visa ändringarna.', 'success');
            } catch (error) {
                showMessage('Misslyckades att uppdatera feed: ' + error.message, 'error');
            }
        });

        // Collect filters from form
        function collectFilters() {
            const filters = {};

            // Grade filter
            const grad = document.getElementById('gradFilter').value;
            if (grad) filters.Grad = grad;

            // Lodge filter - send UNCHECKED lodges (the ones to exclude/remove)
            const uncheckedLodges = Array.from(document.querySelectorAll('#lodgeCheckboxes input:not(:checked)'))
                .map(cb => cb.value);
            if (uncheckedLodges.length > 0) filters.Loge = uncheckedLodges.join(',');

            // Special filters
            if (document.getElementById('removeUnconfirmed').checked) {
                filters.RemoveUnconfirmed = 'true';
            }
            if (document.getElementById('removeInstallt').checked) {
                filters.RemoveInstallt = 'true';
            }

            return filters;
        }

        // Build query string
        function buildQueryString(filters) {
            return Object.entries(filters)
                .map(([key, value]) => encodeURIComponent(key) + '=' + encodeURIComponent(value))
                .join('&');
        }

        // Show message
        function showMessage(text, type) {
            const msg = document.getElementById('message');
            msg.textContent = text;
            msg.className = 'message ' + type;
            setTimeout(() => {
                msg.className = 'message';
            }, 5000);
        }

        // Select all lodges
        function selectAllLodges() {
            document.querySelectorAll('#lodgeCheckboxes input[type="checkbox"]').forEach(cb => {
                cb.checked = true;
            });
        }

        // Deselect all lodges
        function deselectAllLodges() {
            document.querySelectorAll('#lodgeCheckboxes input[type="checkbox"]').forEach(cb => {
                cb.checked = false;
            });
        }

        // Escape HTML
        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }
    </script>
</body>
</html>
`
