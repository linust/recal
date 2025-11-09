package server

const configPageTemplate = `<!DOCTYPE html>
<html lang="sv">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>ReCal - Konfigurera</title>
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      max-width: 800px;
      margin: 40px auto;
      padding: 20px;
      background: #f5f5f5;
    }
    .container {
      background: white;
      padding: 30px;
      border-radius: 8px;
      box-shadow: 0 2px 10px rgba(0,0,0,0.1);
    }
    h1 {
      color: #333;
      margin-bottom: 10px;
    }
    .subtitle {
      color: #666;
      margin-bottom: 30px;
    }
    .filter-section {
      margin-bottom: 30px;
      padding: 20px;
      background: #f9f9f9;
      border-radius: 5px;
    }
    .filter-section h3 {
      margin-top: 0;
      color: #444;
    }
    select {
      width: 100%;
      padding: 10px;
      border: 1px solid #ddd;
      border-radius: 4px;
      font-size: 16px;
    }
    .checkbox-list {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
      gap: 10px;
      margin-top: 15px;
    }
    .checkbox-list label {
      display: flex;
      align-items: center;
      gap: 8px;
    }
    .controls {
      margin-bottom: 15px;
    }
    .controls button {
      padding: 8px 15px;
      margin-right: 10px;
      border: 1px solid #ddd;
      background: white;
      border-radius: 4px;
      cursor: pointer;
    }
    .controls button:hover {
      background: #f0f0f0;
    }
    .url-display {
      background: #f4f4f4;
      padding: 15px;
      border: 1px solid #ddd;
      border-radius: 4px;
      font-family: monospace;
      word-break: break-all;
      margin-bottom: 20px;
    }
    .action-buttons {
      display: flex;
      gap: 15px;
    }
    .action-buttons button {
      flex: 1;
      padding: 15px;
      font-size: 16px;
      border: none;
      border-radius: 4px;
      cursor: pointer;
      font-weight: 500;
    }
    .btn-primary {
      background: #0066cc;
      color: white;
    }
    .btn-primary:hover {
      background: #0052a3;
    }
    .btn-secondary {
      background: #28a745;
      color: white;
    }
    .btn-secondary:hover {
      background: #218838;
    }
    .future-section {
      margin-top: 30px;
      padding: 20px;
      background: #fff3cd;
      border: 1px solid #ffc107;
      border-radius: 4px;
    }
    .future-section h4 {
      margin-top: 0;
      color: #856404;
    }
    .help-text {
      font-size: 14px;
      color: #666;
      margin-top: 10px;
    }
    .loading {
      text-align: center;
      padding: 20px;
      color: #666;
    }
  </style>
</head>
<body>
  <div class="container">
    <h1>ReCal</h1>
    <p class="subtitle">Konfigurera dina kalenderfilter</p>

    <!-- Grade Filter -->
    <div class="filter-section">
      <h3>Grad</h3>
      <select id="grad-select">
        <option value="">Alla grader</option>
        <option value="1">Grad 1</option>
        <option value="2">Grad 2</option>
        <option value="3">Grad 3</option>
        <option value="4">Grad 4</option>
        <option value="5">Grad 5</option>
        <option value="6">Grad 6</option>
        <option value="7">Grad 7</option>
        <option value="8">Grad 8</option>
        <option value="9">Grad 9</option>
        <option value="10">Grad 10</option>
      </select>
      <p class="help-text">
        Behåller vald grad och lägre, filtrerar bort högre grader
      </p>
    </div>

    <!-- Lodge Filter -->
    <div class="filter-section">
      <h3>Loger</h3>
      <div class="controls">
        <button id="select-all-lodges">Välj alla</button>
        <button id="deselect-all-lodges">Avmarkera alla</button>
      </div>
      <div class="checkbox-list" id="loge-checkboxes">
        <div class="loading">Laddar loger...</div>
      </div>
      <p class="help-text">
        Avmarkera loger för att filtrera bort dem
      </p>
    </div>

    <!-- Special Filters -->
    <div class="filter-section">
      <h3>Specialfilter</h3>
      <div class="checkbox-list">
        <label>
          <input type="checkbox" id="remove-unconfirmed">
          Ta bort obekräftade händelser
        </label>
        <label>
          <input type="checkbox" id="remove-installt">
          Ta bort inställda händelser
        </label>
      </div>
    </div>

    <!-- Generated URL -->
    <h3>Genererad URL</h3>
    <div class="url-display" id="generated-url">
      {{.BaseURL}}/filter
    </div>

    <!-- Action Buttons -->
    <div class="action-buttons">
      <button id="copy-url-btn" class="btn-primary">
        📋 Kopiera URL
      </button>
      <button id="download-ical-btn" class="btn-secondary">
        📥 Ladda ner iCal
      </button>
      <button id="preview-btn" class="btn-secondary">
        🔍 Förhandsgranska
      </button>
    </div>

    <!-- Calendar App Integration -->
    <div class="filter-section">
      <h3>Öppna i kalenderapp</h3>
      <div class="action-buttons" id="calendar-apps">
        <!-- Populated by JavaScript based on platform detection -->
      </div>
      <p class="help-text">
        Välj en kalenderapp för att prenumerera på den filtrerade kalendern
      </p>
    </div>
  </div>

  <script>
    const BASE_URL = '{{.BaseURL}}';

    // Load lodges from API
    async function loadLodges() {
      try {
        const response = await fetch('/api/lodges');
        const data = await response.json();

        const container = document.getElementById('loge-checkboxes');
        container.innerHTML = '';

        data.lodges.forEach(lodge => {
          const label = document.createElement('label');
          const checkbox = document.createElement('input');
          checkbox.type = 'checkbox';
          checkbox.value = lodge;
          checkbox.checked = true;
          checkbox.addEventListener('change', generateURL);

          label.appendChild(checkbox);
          label.appendChild(document.createTextNode(' ' + lodge));
          container.appendChild(label);
        });

        // Apply URL parameters if present
        applyURLParameters();
        generateURL();
      } catch (err) {
        document.getElementById('loge-checkboxes').innerHTML =
          '<div class="loading">Kunde inte ladda loger: ' + err.message + '</div>';
      }
    }

    // Parse URL parameters and apply to form
    function applyURLParameters() {
      const params = new URLSearchParams(window.location.search);

      // Apply Grad parameter
      if (params.has('Grad')) {
        document.getElementById('grad-select').value = params.get('Grad');
      }

      // Apply Loge parameter (unchecked lodges)
      if (params.has('Loge')) {
        const uncheckedLodges = params.get('Loge').split(',');
        document.querySelectorAll('#loge-checkboxes input[type="checkbox"]').forEach(cb => {
          if (uncheckedLodges.includes(cb.value)) {
            cb.checked = false;
          }
        });
      }

      // Apply RemoveUnconfirmed parameter
      if (params.has('RemoveUnconfirmed')) {
        document.getElementById('remove-unconfirmed').checked = true;
      }

      // Apply RemoveInstallt parameter
      if (params.has('RemoveInstallt')) {
        document.getElementById('remove-installt').checked = true;
      }
    }

    // Generate URL based on current settings
    function generateURL() {
      const baseURL = BASE_URL + '/filter';
      const params = new URLSearchParams();

      // Add Grad filter
      const grad = document.getElementById('grad-select').value;
      if (grad) params.append('Grad', grad);

      // Add Loge filter (unchecked lodges)
      const uncheckedLodges = Array.from(
        document.querySelectorAll('#loge-checkboxes input[type="checkbox"]:not(:checked)')
      ).map(cb => cb.value);
      if (uncheckedLodges.length > 0) {
        params.append('Loge', uncheckedLodges.join(','));
      }

      // Add special filters (presence-only parameters, no value needed)
      if (document.getElementById('remove-unconfirmed').checked) {
        params.append('RemoveUnconfirmed', '');
      }
      if (document.getElementById('remove-installt').checked) {
        params.append('RemoveInstallt', '');
      }

      const url = params.toString() ? baseURL + '?' + params : baseURL;
      document.getElementById('generated-url').textContent = url;
      return url;
    }

    // Update URL on any input change
    document.getElementById('grad-select').addEventListener('change', generateURL);
    document.getElementById('remove-unconfirmed').addEventListener('change', generateURL);
    document.getElementById('remove-installt').addEventListener('change', generateURL);

    // Copy URL button
    document.getElementById('copy-url-btn').addEventListener('click', () => {
      const url = document.getElementById('generated-url').textContent;
      navigator.clipboard.writeText(url).then(() => {
        alert('URL kopierad till urklipp!');
      }).catch(err => {
        alert('Kunde inte kopiera URL: ' + err);
      });
    });

    // Download iCal button
    document.getElementById('download-ical-btn').addEventListener('click', () => {
      const url = generateURL();
      window.location.href = url;
    });

    // Select/Deselect All buttons
    document.getElementById('select-all-lodges').addEventListener('click', () => {
      document.querySelectorAll('#loge-checkboxes input[type="checkbox"]')
        .forEach(cb => cb.checked = true);
      generateURL();
    });

    document.getElementById('deselect-all-lodges').addEventListener('click', () => {
      document.querySelectorAll('#loge-checkboxes input[type="checkbox"]')
        .forEach(cb => cb.checked = false);
      generateURL();
    });

    // Preview button - open in debug/preview mode
    document.getElementById('preview-btn').addEventListener('click', () => {
      const currentURL = new URL(generateURL());
      const previewURL = currentURL.origin + '/filter/preview' + currentURL.search;
      window.open(previewURL, '_blank');
    });

    // Platform detection
    function detectPlatform() {
      const ua = navigator.userAgent;
      const platform = navigator.platform;

      return {
        isMac: /Mac/.test(platform),
        isIOS: /iPhone|iPad|iPod/.test(platform),
        isWindows: /Win/.test(platform),
        isAndroid: /Android/.test(ua)
      };
    }

    // Generate calendar app buttons based on platform
    function setupCalendarApps() {
      const platform = detectPlatform();
      const container = document.getElementById('calendar-apps');
      container.innerHTML = '';

      // Get current filter URL
      function getSubscriptionURL() {
        const currentURL = generateURL();
        // Convert https:// to webcal:// for calendar subscription
        return currentURL.replace(/^https?:\/\//, 'webcal://');
      }

      function getHTTPSURL() {
        return generateURL();
      }

      // Apple Calendar (macOS/iOS)
      if (platform.isMac || platform.isIOS) {
        const btn = document.createElement('button');
        btn.className = 'btn-secondary';
        btn.innerHTML = '📅 Apple Calendar';
        btn.onclick = () => {
          window.location.href = getSubscriptionURL();
        };
        container.appendChild(btn);
      }

      // Google Calendar (all platforms)
      const googleBtn = document.createElement('button');
      googleBtn.className = 'btn-secondary';
      googleBtn.innerHTML = '🌐 Google Calendar';
      googleBtn.onclick = () => {
        const httpsURL = encodeURIComponent(getHTTPSURL());
        window.open('https://calendar.google.com/calendar/render?cid=' + httpsURL, '_blank');
      };
      container.appendChild(googleBtn);

      // Outlook.com (all platforms)
      const outlookBtn = document.createElement('button');
      outlookBtn.className = 'btn-secondary';
      outlookBtn.innerHTML = '📧 Outlook.com';
      outlookBtn.onclick = () => {
        const httpsURL = encodeURIComponent(getHTTPSURL());
        window.open('https://outlook.live.com/calendar/0/addfromweb?url=' + httpsURL, '_blank');
      };
      container.appendChild(outlookBtn);

      // Generic webcal link (all platforms)
      const webcalBtn = document.createElement('button');
      webcalBtn.className = 'btn-secondary';
      webcalBtn.innerHTML = '📱 Annan app (webcal://)';
      webcalBtn.onclick = () => {
        const webcalURL = getSubscriptionURL();
        navigator.clipboard.writeText(webcalURL).then(() => {
          alert('webcal:// URL kopierad till urklipp!\\n\\nKlistra in i din kalenderapp.');
        }).catch(() => {
          prompt('Kopiera denna webcal:// URL till din kalenderapp:', webcalURL);
        });
      };
      container.appendChild(webcalBtn);
    }

    // Load lodges and setup calendar apps on page load
    loadLodges();
    setupCalendarApps();
  </script>
</body>
</html>
`
