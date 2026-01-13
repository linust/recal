package server

import (
	"html/template"
	"log"
	"net/http"
)

// AdminPage serves the admin dashboard
func (s *Server) AdminPage(w http.ResponseWriter, r *http.Request) {
	// Record request metrics
	s.requestMetrics.RecordRequest()

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse template
	tmpl, err := template.New("admin").Parse(adminPageTemplate)
	if err != nil {
		http.Error(w, "Failed to parse template", http.StatusInternalServerError)
		log.Printf("Admin template parse error: %v", err)
		return
	}

	data := struct {
		BaseURL string
	}{
		BaseURL: s.cfg.Server.BaseURL,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		log.Printf("Admin template execute error: %v", err)
		return
	}
}

const adminPageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ReCal - Admin Dashboard</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            background: #f5f5f5;
            padding: 20px;
        }

        .container {
            max-width: 1400px;
            margin: 0 auto;
        }

        header {
            background: white;
            padding: 30px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            margin-bottom: 30px;
        }

        h1 {
            color: #333;
            font-size: 32px;
            margin-bottom: 10px;
        }

        .subtitle {
            color: #666;
            font-size: 16px;
        }

        .nav-links {
            display: flex;
            gap: 15px;
            margin-top: 20px;
            padding-top: 20px;
            border-top: 1px solid #eee;
        }

        .nav-link {
            padding: 8px 16px;
            background: #0066cc;
            color: white;
            text-decoration: none;
            border-radius: 4px;
            font-size: 14px;
            transition: background 0.2s;
        }

        .nav-link:hover {
            background: #0052a3;
        }

        .nav-link.secondary {
            background: #6c757d;
        }

        .nav-link.secondary:hover {
            background: #5a6268;
        }

        .action-bar {
            background: white;
            padding: 20px 30px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            margin-bottom: 20px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        .btn {
            padding: 10px 20px;
            background: #28a745;
            color: white;
            border: none;
            border-radius: 4px;
            font-size: 14px;
            cursor: pointer;
            text-decoration: none;
            display: inline-block;
            transition: background 0.2s;
        }

        .btn:hover {
            background: #218838;
        }

        .btn-danger {
            background: #dc3545;
        }

        .btn-danger:hover {
            background: #c82333;
        }

        .btn-small {
            padding: 6px 12px;
            font-size: 13px;
        }

        .search-box {
            padding: 10px;
            border: 1px solid #ddd;
            border-radius: 4px;
            width: 300px;
            font-size: 14px;
        }

        .feeds-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
            gap: 20px;
            margin-bottom: 30px;
        }

        .feed-card {
            background: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            transition: box-shadow 0.2s;
        }

        .feed-card:hover {
            box-shadow: 0 4px 8px rgba(0,0,0,0.15);
        }

        .feed-header {
            display: flex;
            justify-content: space-between;
            align-items: start;
            margin-bottom: 15px;
        }

        .feed-description {
            font-weight: 600;
            color: #333;
            font-size: 16px;
            margin-bottom: 5px;
        }

        .feed-slug {
            font-size: 11px;
            color: #999;
            font-family: 'Courier New', monospace;
            word-break: break-all;
        }

        .feed-meta {
            font-size: 13px;
            color: #666;
            margin: 10px 0;
        }

        .feed-meta-item {
            display: flex;
            justify-content: space-between;
            padding: 5px 0;
        }

        .feed-filters {
            background: #f8f9fa;
            padding: 10px;
            border-radius: 4px;
            margin: 10px 0;
            font-size: 13px;
        }

        .filter-item {
            padding: 3px 0;
            color: #495057;
        }

        .filter-key {
            font-weight: 600;
            color: #0066cc;
        }

        .feed-actions {
            display: flex;
            gap: 8px;
            margin-top: 15px;
            padding-top: 15px;
            border-top: 1px solid #eee;
        }

        .modal {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: rgba(0,0,0,0.5);
            z-index: 1000;
            align-items: center;
            justify-content: center;
        }

        .modal.active {
            display: flex;
        }

        .modal-content {
            background: white;
            padding: 30px;
            border-radius: 8px;
            max-width: 900px;
            width: 90%;
            max-height: 90vh;
            overflow-y: auto;
        }

        .modal-header {
            margin-bottom: 20px;
        }

        .modal-header h2 {
            color: #333;
            font-size: 24px;
        }

        .form-group {
            margin-bottom: 20px;
        }

        .form-group label {
            display: block;
            margin-bottom: 8px;
            font-weight: 600;
            color: #333;
            font-size: 14px;
        }

        .form-group input,
        .form-group textarea {
            width: 100%;
            padding: 10px;
            border: 1px solid #ddd;
            border-radius: 4px;
            font-size: 14px;
            font-family: inherit;
        }

        .form-group textarea {
            min-height: 100px;
            resize: vertical;
        }

        .filter-inputs {
            background: #f8f9fa;
            padding: 15px;
            border-radius: 4px;
        }

        .filter-row {
            display: grid;
            grid-template-columns: 1fr 2fr auto;
            gap: 10px;
            margin-bottom: 10px;
            align-items: center;
        }

        .filter-row input {
            padding: 8px;
            border: 1px solid #ddd;
            border-radius: 4px;
            font-size: 13px;
        }

        .btn-remove {
            background: #dc3545;
            color: white;
            border: none;
            padding: 8px 12px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 13px;
        }

        .btn-add {
            background: #28a745;
            color: white;
            border: none;
            padding: 8px 16px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 13px;
            margin-top: 10px;
        }

        .form-actions {
            display: flex;
            gap: 10px;
            justify-content: flex-end;
            margin-top: 20px;
            padding-top: 20px;
            border-top: 1px solid #eee;
        }

        .btn-cancel {
            background: #6c757d;
            color: white;
            border: none;
            padding: 10px 20px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 14px;
        }

        .error {
            background: #f8d7da;
            color: #721c24;
            padding: 12px;
            border-radius: 4px;
            margin-bottom: 15px;
            border: 1px solid #f5c6cb;
        }

        .success {
            background: #d4edda;
            color: #155724;
            padding: 12px;
            border-radius: 4px;
            margin-bottom: 15px;
            border: 1px solid #c3e6cb;
        }

        .empty-state {
            background: white;
            padding: 60px 30px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            text-align: center;
        }

        .empty-state h3 {
            color: #666;
            font-size: 20px;
            margin-bottom: 10px;
        }

        .empty-state p {
            color: #999;
            margin-bottom: 20px;
        }

        .badge {
            display: inline-block;
            padding: 4px 8px;
            background: #e9ecef;
            color: #495057;
            border-radius: 4px;
            font-size: 12px;
            font-weight: 600;
        }

        .badge.badge-success {
            background: #d4edda;
            color: #155724;
        }

        .badge.badge-info {
            background: #d1ecf1;
            color: #0c5460;
        }

        .copy-btn {
            background: #6c757d;
            color: white;
            border: none;
            padding: 4px 8px;
            border-radius: 3px;
            cursor: pointer;
            font-size: 11px;
            margin-left: 5px;
        }

        .copy-btn:hover {
            background: #5a6268;
        }

        @media (max-width: 768px) {
            .feeds-grid {
                grid-template-columns: 1fr;
            }

            .action-bar {
                flex-direction: column;
                gap: 15px;
                align-items: stretch;
            }

            .search-box {
                width: 100%;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>ReCal Admin Dashboard</h1>
            <p class="subtitle">Manage named feeds, view statistics, and configure the service</p>
            <div class="nav-links">
                <a href="/" class="nav-link secondary">← Back to Config</a>
                <a href="/status" class="nav-link secondary">Status Dashboard</a>
                <a href="/health" class="nav-link secondary">Health Check</a>
                <a href="/api/lodges" class="nav-link secondary">Lodges API</a>
            </div>
        </header>

        <div class="action-bar">
            <input type="text" id="searchBox" class="search-box" placeholder="Search feeds by description or filters...">
            <button class="btn" onclick="showCreateModal()">+ Create New Feed</button>
        </div>

        <div id="message"></div>

        <div id="statsBar" style="background: white; padding: 15px 30px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); margin-bottom: 20px; display: none;">
            <div style="display: flex; gap: 30px; align-items: center; font-size: 14px;">
                <div><strong>Total Feeds:</strong> <span id="totalFeeds">0</span></div>
                <div><strong>Showing:</strong> <span id="showingFeeds">0</span></div>
                <div><strong>Page:</strong> <span id="currentPage">1</span> of <span id="totalPages">1</span></div>
            </div>
        </div>

        <div id="feedsContainer"></div>

        <div id="pagination" style="display: flex; justify-content: center; gap: 10px; margin-top: 30px;"></div>
    </div>

    <!-- Create/Edit Modal -->
    <div id="feedModal" class="modal">
        <div class="modal-content">
            <div class="modal-header">
                <h2 id="modalTitle">Create New Feed</h2>
            </div>
            <div id="modalError"></div>
            <form id="feedForm">
                <input type="hidden" id="feedSlug">
                <div class="form-group" id="slugDisplay" style="display: none;">
                    <label>Feed Slug (permanent URL identifier)</label>
                    <input type="text" id="feedSlugReadonly" readonly style="background: #f5f5f5; cursor: not-allowed; font-family: monospace; font-size: 12px;">
                    <small style="color: #666; margin-top: 5px; display: block;">The slug is the permanent identifier in the feed URL and cannot be changed.</small>
                </div>
                <div class="form-group">
                    <label for="feedDescription">Feed Name * <small style="color: #666; font-weight: normal;">(visible in admin UI only)</small></label>
                    <input type="text" id="feedDescription" required maxlength="500" placeholder="e.g., Linus - Grade 4 Stockholm Calendar">
                </div>
                <div class="form-group">
                    <label>Filters * <small style="color: #666;">(at least one required)</small></label>
                    <div style="background: #e8f4f8; padding: 10px; border-radius: 4px; margin-bottom: 10px; font-size: 13px; color: #0c5460;">
                        <strong>How filters work:</strong> Events matching these filters will be <strong>removed</strong> from the calendar.
                        <ul style="margin: 5px 0 0 20px; padding: 0;">
                            <li><code>Grad: 4</code> removes Grade 5+ events (keeps 1-4)</li>
                            <li><code>Loge: Stockholm</code> removes Stockholm events</li>
                            <li><code>pattern: Meeting</code> removes events with "Meeting" in summary/description</li>
                        </ul>
                    </div>

                    <div style="margin-bottom: 15px;">
                        <label style="display: block; margin-bottom: 5px; font-weight: 600; font-size: 14px;">Grade Filter (Remove events above this grade)</label>
                        <select id="gradFilter" style="width: 100%; padding: 8px; border: 1px solid #ddd; border-radius: 4px;">
                            <option value="">No grade filter</option>
                            <option value="1">Grade 1 (remove 2-10)</option>
                            <option value="2">Grade 2 (remove 3-10)</option>
                            <option value="3">Grade 3 (remove 4-10)</option>
                            <option value="4">Grade 4 (remove 5-10)</option>
                            <option value="5">Grade 5 (remove 6-10)</option>
                            <option value="6">Grade 6 (remove 7-10)</option>
                            <option value="7">Grade 7 (remove 8-10)</option>
                            <option value="8">Grade 8 (remove 9-10)</option>
                            <option value="9">Grade 9 (remove 10)</option>
                            <option value="10">Grade 10 (remove none)</option>
                        </select>
                    </div>

                    <div style="margin-bottom: 15px;">
                        <label style="display: block; margin-bottom: 5px; font-weight: 600; font-size: 14px;">Lodge Filter (Remove events from these lodges)</label>
                        <div id="lodgeCheckboxes" style="display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 8px; padding: 10px; background: #f9f9f9; border-radius: 4px; max-height: 200px; overflow-y: auto;"></div>
                    </div>

                    <div style="margin-bottom: 15px;">
                        <label style="display: block; margin-bottom: 5px; font-weight: 600; font-size: 14px;">Special Filters</label>
                        <div style="padding: 10px; background: #f9f9f9; border-radius: 4px;">
                            <label style="display: flex; align-items: center; gap: 8px; margin-bottom: 8px; cursor: pointer; line-height: 1.4;">
                                <input type="checkbox" id="removeUnconfirmed" style="margin: 0; flex-shrink: 0; width: 16px; height: 16px;">
                                <span>Remove unconfirmed events</span>
                            </label>
                            <label style="display: flex; align-items: center; gap: 8px; cursor: pointer; line-height: 1.4;">
                                <input type="checkbox" id="removeInstallt" style="margin: 0; flex-shrink: 0; width: 16px; height: 16px;">
                                <span>Remove cancelled events (INSTÄLLT)</span>
                            </label>
                        </div>
                    </div>

                    <details style="margin-top: 15px;">
                        <summary style="cursor: pointer; color: #0066cc; font-weight: 600; font-size: 14px;">Advanced: Custom Filters</summary>
                        <div style="margin-top: 10px; padding: 10px; background: #f9f9f9; border-radius: 4px;">
                            <div class="filter-inputs" id="filterInputs"></div>
                            <button type="button" class="btn-add" onclick="addFilterRow()">+ Add Custom Filter</button>
                        </div>
                    </details>
                </div>
                <div class="form-actions">
                    <button type="button" class="btn-cancel" onclick="closeModal()">Cancel</button>
                    <button type="submit" class="btn" id="submitBtn">Create Feed</button>
                </div>
            </form>
        </div>
    </div>

    <script>
        const baseURL = '{{.BaseURL}}';
        let currentPage = 1;
        let totalPages = 1;
        let totalFeeds = 0;
        let searchQuery = '';
        let searchTimeout = null;
        let availableLodges = [];

        // Load feeds on page load
        document.addEventListener('DOMContentLoaded', () => {
            loadFeeds();
            setupSearch();
            loadLodges();
        });

        // Load available lodges from API
        async function loadLodges() {
            try {
                const response = await fetch('/api/lodges');
                if (!response.ok) return;
                const data = await response.json();
                availableLodges = data.lodges || [];
            } catch (error) {
                console.error('Failed to load lodges:', error);
            }
        }

        // Load feeds with pagination
        async function loadFeeds(page = 1, search = '') {
            try {
                let url = '/admin/feeds?page=' + page + '&page_size=50';
                if (search) {
                    url += '&q=' + encodeURIComponent(search);
                }

                const response = await fetch(url);
                if (!response.ok) throw new Error('Failed to load feeds');
                const data = await response.json();

                currentPage = data.page || 1;
                totalPages = data.total_pages || 1;
                totalFeeds = data.total || 0;
                const filtered = data.filtered || 0;
                const feeds = data.feeds || [];

                renderFeeds(feeds);
                updateStats(totalFeeds, filtered, feeds.length, currentPage, totalPages);
                renderPagination();
            } catch (error) {
                showError('Failed to load feeds: ' + error.message);
            }
        }

        // Update stats bar
        function updateStats(total, filtered, showing, page, pages) {
            document.getElementById('totalFeeds').textContent = total;
            document.getElementById('showingFeeds').textContent = showing;
            document.getElementById('currentPage').textContent = page;
            document.getElementById('totalPages').textContent = pages;
            document.getElementById('statsBar').style.display = total > 0 ? 'block' : 'none';
        }

        // Render pagination controls
        function renderPagination() {
            const container = document.getElementById('pagination');
            if (totalPages <= 1) {
                container.innerHTML = '';
                return;
            }

            let html = '';

            // Previous button
            if (currentPage > 1) {
                html += '<button class="btn btn-small" onclick="loadFeeds(' + (currentPage - 1) + ', searchQuery)">← Previous</button>';
            }

            // Page numbers
            const maxPages = 5;
            let startPage = Math.max(1, currentPage - Math.floor(maxPages / 2));
            let endPage = Math.min(totalPages, startPage + maxPages - 1);

            if (endPage - startPage < maxPages - 1) {
                startPage = Math.max(1, endPage - maxPages + 1);
            }

            for (let i = startPage; i <= endPage; i++) {
                const active = i === currentPage ? 'style="background: #0066cc; color: white;"' : '';
                html += '<button class="btn btn-small" ' + active + ' onclick="loadFeeds(' + i + ', searchQuery)">' + i + '</button>';
            }

            // Next button
            if (currentPage < totalPages) {
                html += '<button class="btn btn-small" onclick="loadFeeds(' + (currentPage + 1) + ', searchQuery)">Next →</button>';
            }

            container.innerHTML = html;
        }

        // Render feeds
        function renderFeeds(feedsToRender) {
            const container = document.getElementById('feedsContainer');

            if (feedsToRender.length === 0) {
                container.innerHTML = '<div class="empty-state"><h3>No feeds yet</h3><p>Create your first named feed to get started</p><button class="btn" onclick="showCreateModal()">Create New Feed</button></div>';
                return;
            }

            const html = '<div class="feeds-grid">' + feedsToRender.map(feed => {
                const feedURL = baseURL + '/feed/' + feed.slug;
                const configURL = feedURL + '/config';
                const previewURL = feedURL + '/preview';
                const createdDate = new Date(feed.created_at).toLocaleDateString();
                const lastAccess = feed.last_access ? new Date(feed.last_access).toLocaleString() : 'Never';

                const filterItems = Object.entries(feed.filters || {}).map(([key, value]) =>
                    '<div class="filter-item"><span class="filter-key">' + escapeHtml(key) + '</span>: ' + escapeHtml(value) + '</div>'
                ).join('');

                return '<div class="feed-card">' +
                    '<div class="feed-header">' +
                        '<div style="flex: 1;">' +
                            '<div class="feed-description">' + escapeHtml(feed.description) + '</div>' +
                            '<div class="feed-slug">' + escapeHtml(feed.slug) + '</div>' +
                        '</div>' +
                    '</div>' +
                    '<div class="feed-meta">' +
                        '<div class="feed-meta-item"><span>Created:</span><span>' + createdDate + '</span></div>' +
                        '<div class="feed-meta-item"><span>Access Count:</span><span class="badge badge-info">' + feed.access_count + '</span></div>' +
                        '<div class="feed-meta-item"><span>Last Access:</span><span>' + lastAccess + '</span></div>' +
                    '</div>' +
                    '<div class="feed-filters">' + filterItems + '</div>' +
                    '<div class="feed-actions">' +
                        '<button class="btn btn-small" onclick="copyToClipboard(\'' + feedURL + '\')">Copy URL</button>' +
                        '<button class="btn btn-small" onclick="editFeed(\'' + feed.slug + '\')">Edit</button>' +
                        '<a href="' + previewURL + '" class="btn btn-small" target="_blank">Preview</a>' +
                        '<a href="' + configURL + '" class="btn btn-small" target="_blank" style="background: #6c757d;">Test in Builder</a>' +
                        '<button class="btn btn-small btn-danger" onclick="deleteFeed(\'' + feed.slug + '\')">Delete</button>' +
                    '</div>' +
                '</div>';
            }).join('') + '</div>';

            container.innerHTML = html;
        }

        // Setup search with debouncing
        function setupSearch() {
            const searchBox = document.getElementById('searchBox');
            searchBox.addEventListener('input', (e) => {
                const query = e.target.value.trim();

                // Clear previous timeout
                if (searchTimeout) {
                    clearTimeout(searchTimeout);
                }

                // Debounce search requests (wait 500ms after user stops typing)
                searchTimeout = setTimeout(() => {
                    searchQuery = query;
                    loadFeeds(1, query); // Reset to page 1 on new search
                }, 500);
            });
        }

        // Show create modal
        function showCreateModal() {
            document.getElementById('modalTitle').textContent = 'Create New Feed';
            document.getElementById('submitBtn').textContent = 'Create Feed';
            document.getElementById('feedForm').reset();
            document.getElementById('feedSlug').value = '';
            document.getElementById('modalError').innerHTML = '';

            // Hide slug display for new feeds
            document.getElementById('slugDisplay').style.display = 'none';

            // Reset visual filters
            document.getElementById('gradFilter').value = '';
            document.getElementById('removeUnconfirmed').checked = false;
            document.getElementById('removeInstallt').checked = false;
            populateLodgeCheckboxes([]);

            // Clear custom filters
            document.getElementById('filterInputs').innerHTML = '';

            document.getElementById('feedModal').classList.add('active');
        }

        // Populate lodge checkboxes
        function populateLodgeCheckboxes(selectedLodges = []) {
            const container = document.getElementById('lodgeCheckboxes');
            container.innerHTML = availableLodges.map(lodge => {
                const checked = selectedLodges.includes(lodge) ? 'checked' : '';
                return '<label style="display: flex; align-items: center; gap: 8px; cursor: pointer; font-size: 13px; line-height: 1.4; padding: 2px 0;">' +
                    '<input type="checkbox" value="' + escapeHtml(lodge) + '" ' + checked + ' style="margin: 0; flex-shrink: 0; width: 16px; height: 16px;">' +
                    '<span style="white-space: nowrap;">' + escapeHtml(lodge) + '</span>' +
                    '</label>';
            }).join('');
        }

        // Show edit modal
        async function editFeed(slug) {
            try {
                const response = await fetch('/admin/feeds/' + slug);
                if (!response.ok) throw new Error('Failed to load feed');
                const feed = await response.json();

                document.getElementById('modalTitle').textContent = 'Edit Feed';
                document.getElementById('submitBtn').textContent = 'Save Changes';
                document.getElementById('feedSlug').value = slug;
                document.getElementById('feedDescription').value = feed.description;
                document.getElementById('modalError').innerHTML = '';

                // Show slug display for existing feeds
                document.getElementById('slugDisplay').style.display = 'block';
                document.getElementById('feedSlugReadonly').value = slug;

                // Parse and populate visual filters
                const filters = feed.filters || {};

                // Grade filter
                document.getElementById('gradFilter').value = filters.Grad || '';

                // Lodge filter (comma-separated)
                const selectedLodges = filters.Loge ? filters.Loge.split(',').map(l => l.trim()) : [];
                populateLodgeCheckboxes(selectedLodges);

                // Special filters
                document.getElementById('removeUnconfirmed').checked = filters.RemoveUnconfirmed === 'true' || filters.RemoveUnconfirmed === true;
                document.getElementById('removeInstallt').checked = filters.RemoveInstallt === 'true' || filters.RemoveInstallt === true;

                // Custom filters (everything else)
                const container = document.getElementById('filterInputs');
                container.innerHTML = '';
                const knownFilters = ['Grad', 'Loge', 'RemoveUnconfirmed', 'RemoveInstallt'];
                Object.entries(filters).forEach(([key, value]) => {
                    if (!knownFilters.includes(key)) {
                        addFilterRow(key, value);
                    }
                });

                document.getElementById('feedModal').classList.add('active');
            } catch (error) {
                showError('Failed to load feed: ' + error.message);
            }
        }

        // Close modal
        function closeModal() {
            document.getElementById('feedModal').classList.remove('active');
        }

        // Add filter row
        function addFilterRow(key = '', value = '') {
            const container = document.getElementById('filterInputs');
            const row = document.createElement('div');
            row.className = 'filter-row';
            row.innerHTML =
                '<input type="text" placeholder="Key (e.g., pattern, Grad)" value="' + escapeHtml(key) + '" class="filter-key-input">' +
                '<input type="text" placeholder="Value (e.g., Meeting|Standup)" value="' + escapeHtml(value) + '" class="filter-value-input">' +
                '<button type="button" class="btn-remove" onclick="removeFilterRow(this)">Remove</button>';
            container.appendChild(row);
        }

        // Remove filter row
        function removeFilterRow(btn) {
            btn.parentElement.remove();
        }

        // Submit form
        document.getElementById('feedForm').addEventListener('submit', async (e) => {
            e.preventDefault();

            const slug = document.getElementById('feedSlug').value;
            const description = document.getElementById('feedDescription').value;

            // Collect filters from visual UI
            const filters = {};

            // Grade filter
            const grad = document.getElementById('gradFilter').value;
            if (grad) {
                filters.Grad = grad;
            }

            // Lodge filter (collect checked lodges)
            const checkedLodges = Array.from(document.querySelectorAll('#lodgeCheckboxes input:checked'))
                .map(cb => cb.value);
            if (checkedLodges.length > 0) {
                filters.Loge = checkedLodges.join(',');
            }

            // Special filters
            if (document.getElementById('removeUnconfirmed').checked) {
                filters.RemoveUnconfirmed = 'true';
            }
            if (document.getElementById('removeInstallt').checked) {
                filters.RemoveInstallt = 'true';
            }

            // Custom filters
            document.querySelectorAll('.filter-row').forEach(row => {
                const key = row.querySelector('.filter-key-input').value.trim();
                const value = row.querySelector('.filter-value-input').value.trim();
                if (key && value) {
                    filters[key] = value;
                }
            });

            if (Object.keys(filters).length === 0) {
                showModalError('At least one filter is required');
                return;
            }

            const method = slug ? 'PUT' : 'POST';
            const url = slug ? '/admin/feeds/' + slug : '/admin/feeds';

            try {
                const response = await fetch(url, {
                    method: method,
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ description, filters })
                });

                if (!response.ok) {
                    const error = await response.text();
                    throw new Error(error);
                }

                closeModal();
                await loadFeeds(currentPage, searchQuery);
                showSuccess(slug ? 'Feed updated successfully' : 'Feed created successfully');
            } catch (error) {
                showModalError('Failed to save feed: ' + error.message);
            }
        });

        // Delete feed
        async function deleteFeed(slug) {
            if (!confirm('Are you sure you want to delete this feed? This action cannot be undone.')) {
                return;
            }

            try {
                const response = await fetch('/admin/feeds/' + slug, { method: 'DELETE' });
                if (!response.ok) throw new Error('Failed to delete feed');

                await loadFeeds(currentPage, searchQuery);
                showSuccess('Feed deleted successfully');
            } catch (error) {
                showError('Failed to delete feed: ' + error.message);
            }
        }

        // Copy to clipboard
        function copyToClipboard(text) {
            navigator.clipboard.writeText(text).then(() => {
                showSuccess('URL copied to clipboard!');
            }).catch(err => {
                showError('Failed to copy: ' + err.message);
            });
        }

        // Show error message
        function showError(message) {
            const div = document.getElementById('message');
            div.innerHTML = '<div class="error">' + escapeHtml(message) + '</div>';
            setTimeout(() => div.innerHTML = '', 5000);
        }

        // Show success message
        function showSuccess(message) {
            const div = document.getElementById('message');
            div.innerHTML = '<div class="success">' + escapeHtml(message) + '</div>';
            setTimeout(() => div.innerHTML = '', 3000);
        }

        // Show modal error
        function showModalError(message) {
            document.getElementById('modalError').innerHTML = '<div class="error">' + escapeHtml(message) + '</div>';
        }

        // Escape HTML
        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        // Close modal on outside click
        document.getElementById('feedModal').addEventListener('click', (e) => {
            if (e.target.id === 'feedModal') {
                closeModal();
            }
        });
    </script>
</body>
</html>
`
