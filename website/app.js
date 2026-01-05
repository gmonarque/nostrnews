const { SimplePool } = NostrTools;

const RELAYS = [
  'wss://relay.damus.io',
  'wss://nos.lol',
  'wss://relay.primal.net',
  'wss://relay.snort.social',
  'wss://nostr.land',
  'wss://nostr-pub.wellorder.net',
  'wss://offchain.pub',
  'wss://relay.nostr.band'
];

const NEWS_PUBKEY = '118e01f2686287f96821b037a325c44c13cbda295a2b8ff4440bb1a6b8b61d06';
const BATCH_SIZE = 20;
const KIND_LONG_FORM = 30023;

let pool = null;
let articles = [];
let loading = false;
let endReached = false;
let oldestTimestamp = Math.floor(Date.now() / 1000);
let newestEventTime = 0;
let filters = loadFilters();
let filterOptions = { countries: new Set(), categories: new Set(), tags: new Set(), sources: new Set() };

const $articles = document.getElementById('articles');
const $loading = document.getElementById('loading');
const $endMessage = document.getElementById('end-message');
const $modal = document.getElementById('modal');
const $modalArticle = document.getElementById('modal-article');
const $filterCountry = document.getElementById('filter-country');
const $filterCategory = document.getElementById('filter-category');
const $filterTag = document.getElementById('filter-tag');
const $filterSource = document.getElementById('filter-source');
const $clearFilters = document.getElementById('clear-filters');

const COUNTRY_NAMES = {
  'us': 'United States',
  'uk': 'United Kingdom',
  'au': 'Australia',
  'ca': 'Canada',
  'de': 'Germany',
  'fr': 'France',
  'es': 'Spain',
  'it': 'Italy',
  'jp': 'Japan',
  'mx': 'Mexico',
  'br': 'Brazil',
  'pl': 'Poland',
  'ie': 'Ireland',
  'za': 'South Africa',
  'ua': 'Ukraine',
  'hk': 'Hong Kong',
  'international': 'International'
};

async function init() {
  pool = new SimplePool();
  applyFiltersToUI();

  $filterCountry.addEventListener('change', onFilterChange);
  $filterCategory.addEventListener('change', onFilterChange);
  $filterTag.addEventListener('change', onFilterChange);
  $filterSource.addEventListener('change', onFilterChange);
  $clearFilters.addEventListener('click', clearFilters);
  $modal.addEventListener('click', closeModal);
  document.querySelector('.modal-close').addEventListener('click', closeModal);
  document.querySelector('.modal-content').addEventListener('click', e => e.stopPropagation());
  window.addEventListener('scroll', onScroll);

  await fetchArticles();
  setInterval(fetchNewArticles, 60000);
}

async function fetchNewArticles() {
  if (loading || newestEventTime === 0) return;

  try {
    const events = await pool.querySync(RELAYS, {
      kinds: [KIND_LONG_FORM],
      authors: [NEWS_PUBKEY],
      since: newestEventTime + 1,
      limit: 50
    });

    if (events.length === 0) return;

    let newCount = 0;
    for (const event of events) {
      const article = parseArticle(event);
      if (article && !articles.find(a => a.id === article.id)) {
        articles.unshift(article);
        collectFilterOptions(article);
        newCount++;
        if (article.eventTime > newestEventTime) {
          newestEventTime = article.eventTime;
        }
      }
    }

    if (newCount > 0) {
      articles.sort((a, b) => b.published - a.published);
      updateFilterDropdowns();
      if ($modal.classList.contains('hidden')) {
        renderArticles();
      }
      showNewArticlesBadge(newCount);
    }
  } catch (err) {
    console.error('Auto-refresh error:', err);
  }
}

function showNewArticlesBadge(count) {
  const badge = document.createElement('div');
  badge.className = 'new-articles-badge';
  badge.textContent = `${count} new article${count > 1 ? 's' : ''}`;
  document.body.appendChild(badge);
  requestAnimationFrame(() => badge.classList.add('visible'));
  setTimeout(() => {
    badge.classList.remove('visible');
    setTimeout(() => badge.remove(), 300);
  }, 3000);
}

async function fetchArticles() {
  if (loading || endReached) return;

  loading = true;
  $loading.classList.remove('hidden');

  try {
    const events = await pool.querySync(RELAYS, {
      kinds: [KIND_LONG_FORM],
      authors: [NEWS_PUBKEY],
      until: oldestTimestamp - 1,
      limit: BATCH_SIZE
    });

    if (events.length === 0) {
      endReached = true;
      $endMessage.classList.remove('hidden');
    } else {
      events.sort((a, b) => b.created_at - a.created_at);
      const oldest = events[events.length - 1];
      if (oldest) oldestTimestamp = oldest.created_at;

      for (const event of events) {
        const article = parseArticle(event);
        if (article && !articles.find(a => a.id === article.id)) {
          articles.push(article);
          collectFilterOptions(article);
          if (article.eventTime > newestEventTime) {
            newestEventTime = article.eventTime;
          }
        }
      }

      updateFilterDropdowns();
      renderArticles();
    }
  } catch (err) {
    console.error('Fetch error:', err);
  } finally {
    loading = false;
    $loading.classList.add('hidden');
  }
}

function parseArticle(event) {
  const tags = event.tags || [];
  const getTag = (name) => {
    const tag = tags.find(t => t[0] === name);
    return tag ? tag[1] : '';
  };
  const getTags = (name) => tags.filter(t => t[0] === name).map(t => t[1]);

  return {
    id: event.id,
    eventTime: event.created_at,
    title: getTag('title') || 'Untitled',
    summary: getTag('summary') || '',
    content: event.content || '',
    link: getTag('r') || '',
    image: getTag('image') || '',
    published: parseInt(getTag('published_at')) || event.created_at,
    country: getTag('country') || '',
    category: getTag('category') || '',
    paywall: getTag('paywall') || 'none',
    source: getTag('source') || '',
    archive: getTag('archive') || '',
    author: getTag('author') || '',
    hashtags: getTags('t')
  };
}

function collectFilterOptions(article) {
  if (article.country) filterOptions.countries.add(article.country);
  if (article.category) filterOptions.categories.add(article.category);
  if (article.source) filterOptions.sources.add(article.source);
  for (const tag of article.hashtags) {
    filterOptions.tags.add(tag);
  }
}

function updateFilterDropdowns() {
  updateDropdown($filterCountry, filterOptions.countries, filters.country, true);
  updateDropdown($filterCategory, filterOptions.categories, filters.category);
  updateDropdown($filterTag, filterOptions.tags, filters.tag);
  updateDropdown($filterSource, filterOptions.sources, filters.source);
}

function updateDropdown($select, options, currentValue, isCountry = false) {
  while ($select.options.length > 1) $select.remove(1);

  const sorted = Array.from(options).sort((a, b) => {
    const nameA = isCountry ? getCountryName(a) : a;
    const nameB = isCountry ? getCountryName(b) : b;
    return nameA.localeCompare(nameB);
  });

  for (const opt of sorted) {
    const $opt = document.createElement('option');
    $opt.value = opt;
    $opt.textContent = isCountry ? getCountryName(opt) : truncate(opt, 15).toUpperCase();
    $select.appendChild($opt);
  }

  if (currentValue) $select.value = currentValue;
}

function renderArticles() {
  const filtered = articles.filter(matchesFilters);
  $articles.innerHTML = '';

  for (const article of filtered) {
    const $card = document.createElement('div');
    $card.className = 'article-card';
    $card.innerHTML = `
      ${article.image ? `<div class="article-image"><img src="${escapeHtml(article.image)}" alt="" loading="lazy"></div>` : ''}
      <div class="article-body">
        <div class="meta">
          <span>${formatDate(article.published)}</span>
          ${article.country ? `<span>${getCountryName(article.country)}</span>` : ''}
          <a class="njump-btn" href="https://njump.me/${article.id}" target="_blank" rel="noopener" title="View on Nostr">n</a>
        </div>
        <div class="title">${escapeHtml(article.title)}</div>
        ${article.summary ? `<div class="summary">${escapeHtml(article.summary)}</div>` : ''}
        <div class="card-footer">
          <span class="source-link" data-source="${escapeHtml(article.source)}">${escapeHtml(article.source)}</span>
          <div class="tags">${article.category ? `<span class="tag" data-category="${escapeHtml(article.category)}">${escapeHtml(article.category)}</span>` : ''}${article.hashtags.map(t => `<span class="tag" data-tag="${escapeHtml(t)}">${escapeHtml(t)}</span>`).join('')}</div>
        </div>
      </div>
    `;
    $card.addEventListener('click', (e) => {
      if (e.target.classList.contains('njump-btn')) {
        e.stopPropagation();
        return;
      }
      if (e.target.classList.contains('source-link')) {
        e.stopPropagation();
        setSourceFilter(e.target.dataset.source);
        return;
      }
      if (e.target.classList.contains('tag')) {
        e.stopPropagation();
        const tag = e.target.dataset.tag;
        const category = e.target.dataset.category;
        if (tag) setTagFilter(tag);
        else if (category) setCategoryFilter(category);
      } else {
        openModal(article);
      }
    });
    $articles.appendChild($card);
  }
}

function matchesFilters(article) {
  if (filters.country && article.country !== filters.country) return false;
  if (filters.category && article.category !== filters.category) return false;
  if (filters.tag && !article.hashtags.includes(filters.tag)) return false;
  if (filters.source && article.source !== filters.source) return false;
  return true;
}

function onFilterChange() {
  filters = {
    country: $filterCountry.value,
    category: $filterCategory.value,
    tag: $filterTag.value,
    source: $filterSource.value
  };
  saveFilters();
  renderArticles();
}

function clearFilters() {
  $filterCountry.value = '';
  $filterCategory.value = '';
  $filterTag.value = '';
  $filterSource.value = '';
  filters = { country: '', category: '', tag: '', source: '' };
  saveFilters();
  renderArticles();
}

function setTagFilter(tag) {
  $filterTag.value = tag;
  filters.tag = tag;
  saveFilters();
  renderArticles();
  window.scrollTo(0, 0);
}

function setCategoryFilter(category) {
  $filterCategory.value = category;
  filters.category = category;
  saveFilters();
  renderArticles();
  window.scrollTo(0, 0);
}

function setSourceFilter(source) {
  $filterSource.value = source;
  filters.source = source;
  saveFilters();
  renderArticles();
  window.scrollTo(0, 0);
}

function applyFiltersToUI() {
  $filterCountry.value = filters.country || '';
  $filterCategory.value = filters.category || '';
  $filterTag.value = filters.tag || '';
  $filterSource.value = filters.source || '';
}

function saveFilters() {
  localStorage.setItem('nostr-news-filters', JSON.stringify(filters));
}

function loadFilters() {
  try {
    const saved = JSON.parse(localStorage.getItem('nostr-news-filters'));
    if (saved) return saved;
  } catch { }
  return {};
}

function openModal(article) {
  let content = article.content.replace(/^\*\*Source:\*\*.*?\n\n---\n\n/s, '');

  $modalArticle.innerHTML = `
    ${article.image ? `<div class="modal-image"><img src="${escapeHtml(article.image)}" alt=""></div>` : ''}
    <div class="meta">
      <span>${formatDate(article.published)}</span>
      ${article.country ? `<span>${getCountryName(article.country)}</span>` : ''}
    </div>
    <h2 class="title">${escapeHtml(article.title)}</h2>
    ${article.author ? `<div class="author">By ${escapeHtml(article.author)}</div>` : ''}
    ${article.paywall !== 'none' ? `<div class="paywall-notice">This is probably paywalled, you can try the archive link (it's slow to load, just wait).</div>` : ''}
    <div class="links">
      ${article.link ? `<a class="source-link" href="${escapeHtml(article.link)}" target="_blank" rel="noopener">${escapeHtml(article.source)} &rarr;</a>` : ''}
      ${article.archive ? `<a class="archive-link" href="${escapeHtml(article.archive)}" target="_blank" rel="noopener">archive &rarr;</a>` : ''}
    </div>
    <div class="content">${formatContent(content)}</div>
    <div class="tags">
      ${article.category ? `<span class="tag" data-category="${escapeHtml(article.category)}">${escapeHtml(article.category)}</span>` : ''}${article.hashtags.map(t => `<span class="tag" data-tag="${escapeHtml(t)}">${escapeHtml(t)}</span>`).join('')}
    </div>
  `;

  $modalArticle.querySelectorAll('.tag[data-tag]').forEach(el => {
    el.addEventListener('click', () => {
      closeModalDirect();
      setTagFilter(el.dataset.tag);
    });
  });
  $modalArticle.querySelectorAll('.tag[data-category]').forEach(el => {
    el.addEventListener('click', () => {
      closeModalDirect();
      setCategoryFilter(el.dataset.category);
    });
  });

  $modal.classList.remove('hidden');
  document.body.style.overflow = 'hidden';
}

function closeModal(e) {
  if (e.target === $modal || e.target.classList.contains('modal-close')) {
    closeModalDirect();
  }
}

function closeModalDirect() {
  $modal.classList.add('hidden');
  document.body.style.overflow = '';
  renderArticles();
}

function onScroll() {
  if (window.scrollY + window.innerHeight >= document.documentElement.scrollHeight - 500) {
    fetchArticles();
  }
}

function getCountryName(code) {
  if (!code) return '';
  return COUNTRY_NAMES[code.toLowerCase()] || code.toUpperCase();
}

function formatDate(timestamp) {
  const d = new Date(timestamp * 1000);
  const pad = n => n.toString().padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function escapeHtml(str) {
  if (!str) return '';
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

function truncate(str, maxLen) {
  if (!str || str.length <= maxLen) return str;
  return str.slice(0, maxLen - 1) + '…';
}

function formatContent(content) {
  if (!content) return '';
  let html = escapeHtml(content);
  html = html.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');
  html = html.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
  return html;
}

document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape' && !$modal.classList.contains('hidden')) {
    closeModalDirect();
  }
});

init();
