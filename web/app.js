// Earworm — Frontend Logic
// Vanilla JS: WebSocket real-time, REST API, card rendering, voting, sorting

(function () {
  'use strict';

  // --- DOM References ---
  var postForm = document.getElementById('post-form');
  var youtubeUrlInput = document.getElementById('youtube-url');
  var thoughtInput = document.getElementById('thought');
  var charCount = document.getElementById('char-count');
  var submitBtn = document.getElementById('submit-btn');
  var cardGrid = document.getElementById('card-grid');
  var connectionStatus = document.getElementById('connection-status');
  var viewerCount = document.getElementById('viewer-count');
  var sortDateBtn = document.getElementById('sort-date');
  var sortLikesBtn = document.getElementById('sort-likes');
  var sortShuffleBtn = document.getElementById('sort-shuffle');

  // --- State ---
  var allVibes = [];
  var sortMode = 'date'; // 'date' or 'likes'

  // --- WebSocket State ---
  var ws = null;
  var reconnectDelay = 1000;
  var MAX_RECONNECT_DELAY = 30000;
  var reconnectTimer = null;

  // --- localStorage Vote Tracker ---
  function getVoteTracker() {
    try {
      var data = localStorage.getItem('earworm_votes');
      return data ? JSON.parse(data) : {};
    } catch (e) {
      return {};
    }
  }

  function setVoteTracker(tracker) {
    try {
      localStorage.setItem('earworm_votes', JSON.stringify(tracker));
    } catch (e) {
      // localStorage unavailable — degrade gracefully
    }
  }

  function hasVoted(vibeId) {
    var tracker = getVoteTracker();
    return tracker[String(vibeId)] || null;
  }

  function recordVote(vibeId, direction) {
    var tracker = getVoteTracker();
    tracker[String(vibeId)] = direction;
    setVoteTracker(tracker);
  }

  // --- Relative Time ---
  function relativeTime(isoString) {
    var now = Date.now();
    var then = new Date(isoString).getTime();
    var diffSeconds = Math.floor((now - then) / 1000);

    if (diffSeconds < 60) {
      return diffSeconds + 's ago';
    }
    var diffMinutes = Math.floor(diffSeconds / 60);
    if (diffMinutes < 60) {
      return diffMinutes + 'm ago';
    }
    var diffHours = Math.floor(diffMinutes / 60);
    if (diffHours < 24) {
      return diffHours + 'h ago';
    }
    var diffDays = Math.floor(diffHours / 24);
    return diffDays + 'd ago';
  }

  // Update all time labels every 30 seconds
  setInterval(function () {
    var timeEls = cardGrid.querySelectorAll('.card-time[data-created-at]');
    timeEls.forEach(function (el) {
      el.textContent = relativeTime(el.getAttribute('data-created-at'));
    });
  }, 30000);

  // --- Helpers ---
  function extractVideoID(url) {
    if (url.indexOf('youtube.com/watch?v=') !== -1) {
      return url.split('watch?v=')[1].substring(0, 11);
    } else if (url.indexOf('youtu.be/') !== -1) {
      return url.split('youtu.be/')[1].substring(0, 11);
    } else if (url.indexOf('youtube.com/shorts/') !== -1) {
      return url.split('shorts/')[1].substring(0, 11);
    }
    return null;
  }

  // --- Sorting ---
  function sortVibes(vibes, mode) {
    if (mode === 'date') {
      return vibes.sort(function (a, b) {
        return new Date(b.created_at) - new Date(a.created_at);
      });
    }
    // mode === 'likes'
    return vibes.sort(function (a, b) {
      if (b.net_score !== a.net_score) return b.net_score - a.net_score;
      return new Date(b.created_at) - new Date(a.created_at);
    });
  }

  function resortAndRender() {
    sortVibes(allVibes, sortMode);
    renderVibes(allVibes);
  }

  // --- Vote Handler ---
  function handleVote(vibeId, direction) {
    var previousVote = hasVoted(vibeId);
    if (previousVote === direction) return; // already voted this way

    // Disable buttons on this card
    var card = cardGrid.querySelector('[data-vibe-id="' + vibeId + '"]');
    var buttons = card ? card.querySelectorAll('.vote-btn') : [];
    buttons.forEach(function (btn) { btn.disabled = true; });

    fetch('/api/vibes/' + vibeId + '/vote', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ direction: direction })
    }).then(function (res) {
      if (!res.ok) throw new Error('Vote failed');
      return res.json();
    }).then(function (vibe) {
      recordVote(vibeId, direction);
      // Update local state
      updateVibeInArray(vibe);
      // Update button active states
      if (card) {
        updateVoteButtonStates(card, vibeId);
        updateNetScoreDisplay(card, vibe.net_score);
      }
      // Re-enable buttons
      buttons.forEach(function (btn) { btn.disabled = false; });
    }).catch(function () {
      // Re-enable buttons on failure
      buttons.forEach(function (btn) { btn.disabled = false; });
    });
  }

  function updateVibeInArray(updatedVibe) {
    for (var i = 0; i < allVibes.length; i++) {
      if (allVibes[i].id === updatedVibe.id) {
        allVibes[i].likes = updatedVibe.likes;
        allVibes[i].dislikes = updatedVibe.dislikes;
        allVibes[i].net_score = updatedVibe.net_score;
        break;
      }
    }
  }

  function updateVoteButtonStates(card, vibeId) {
    var previousVote = hasVoted(vibeId);
    var upBtn = card.querySelector('.upvote');
    var downBtn = card.querySelector('.downvote');
    if (upBtn) {
      upBtn.classList.toggle('active', previousVote === 'up');
    }
    if (downBtn) {
      downBtn.classList.toggle('active', previousVote === 'down');
    }
  }

  function updateNetScoreDisplay(card, netScore) {
    var scoreEl = card.querySelector('.net-score');
    if (scoreEl) {
      scoreEl.textContent = netScore;
    }
  }

  // --- Card Rendering ---
  function createCardElement(vibe) {
    var card = document.createElement('article');
    card.className = 'vibe-card';
    card.setAttribute('aria-label', 'Earworm vibe');
    card.setAttribute('data-vibe-id', vibe.id);

    // YouTube Embed (starts at 0:45)
    var thumbDiv = document.createElement('div');
    thumbDiv.className = 'card-thumbnail';
    var videoID = extractVideoID(vibe.youtube_url);
    if (videoID) {
      var iframe = document.createElement('iframe');
      iframe.src = 'https://www.youtube.com/embed/' + videoID + '?start=45';
      iframe.title = vibe.video_title || 'YouTube video';
      iframe.setAttribute('frameborder', '0');
      iframe.setAttribute('allow', 'accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture');
      iframe.setAttribute('allowfullscreen', '');
      iframe.loading = 'lazy';
      thumbDiv.appendChild(iframe);
    } else {
      thumbDiv.classList.add('no-thumbnail');
      thumbDiv.textContent = '🎵 No video';
    }
    card.appendChild(thumbDiv);

    // Title
    var titleDiv = document.createElement('div');
    titleDiv.className = 'card-title';
    titleDiv.textContent = vibe.video_title || 'Untitled';
    card.appendChild(titleDiv);

    // Thought
    var thoughtDiv = document.createElement('div');
    thoughtDiv.className = 'card-thought';
    thoughtDiv.textContent = vibe.thought;
    card.appendChild(thoughtDiv);

    // Author attribution
    var authorDiv = document.createElement('div');
    authorDiv.className = 'card-author';
    authorDiv.textContent = '— ' + (vibe.author || 'Anonymous');
    card.appendChild(authorDiv);

    // Vote Controls
    var voteControls = document.createElement('div');
    voteControls.className = 'vote-controls';

    var upBtn = document.createElement('button');
    upBtn.className = 'vote-btn upvote';
    upBtn.textContent = '▲';
    upBtn.setAttribute('aria-label', 'Upvote');
    upBtn.addEventListener('click', function () {
      handleVote(vibe.id, 'up');
    });

    var scoreSpan = document.createElement('span');
    scoreSpan.className = 'net-score';
    scoreSpan.textContent = vibe.net_score || 0;

    var downBtn = document.createElement('button');
    downBtn.className = 'vote-btn downvote';
    downBtn.textContent = '▼';
    downBtn.setAttribute('aria-label', 'Downvote');
    downBtn.addEventListener('click', function () {
      handleVote(vibe.id, 'down');
    });

    // Check vote tracker and highlight active button
    var previousVote = hasVoted(vibe.id);
    if (previousVote === 'up') {
      upBtn.classList.add('active');
    } else if (previousVote === 'down') {
      downBtn.classList.add('active');
    }

    voteControls.appendChild(upBtn);
    voteControls.appendChild(scoreSpan);
    voteControls.appendChild(downBtn);
    card.appendChild(voteControls);

    // Relative time
    var timeDiv = document.createElement('div');
    timeDiv.className = 'card-time';
    timeDiv.setAttribute('data-created-at', vibe.created_at);
    timeDiv.textContent = relativeTime(vibe.created_at);
    card.appendChild(timeDiv);

    return card;
  }

  function renderVibes(vibes) {
    cardGrid.innerHTML = '';
    if (vibes.length === 0) {
      var empty = document.createElement('div');
      empty.className = 'empty-state';
      empty.innerHTML = '<p>🎵</p><p>No earworms yet. Share what\'s stuck in your head!</p>';
      cardGrid.appendChild(empty);
      return;
    }
    vibes.forEach(function (vibe) {
      cardGrid.appendChild(createCardElement(vibe));
    });
  }

  function prependVibe(vibe) {
    // Remove empty state if present
    var empty = cardGrid.querySelector('.empty-state');
    if (empty) {
      empty.remove();
    }
    var card = createCardElement(vibe);
    card.classList.add('slide-in');
    cardGrid.prepend(card);
  }

  function insertVibeAtSortedPosition(vibe) {
    // Remove empty state if present
    var empty = cardGrid.querySelector('.empty-state');
    if (empty) {
      empty.remove();
    }

    var card = createCardElement(vibe);
    card.classList.add('slide-in');

    // Find the correct position in the existing cards
    var cards = cardGrid.querySelectorAll('.vibe-card');
    var inserted = false;
    for (var i = 0; i < cards.length; i++) {
      var cardVibeId = parseInt(cards[i].getAttribute('data-vibe-id'), 10);
      var existingVibe = findVibeById(cardVibeId);
      if (existingVibe) {
        // Compare by net_score DESC, then created_at DESC
        if (vibe.net_score > existingVibe.net_score ||
            (vibe.net_score === existingVibe.net_score &&
             new Date(vibe.created_at) > new Date(existingVibe.created_at))) {
          cardGrid.insertBefore(card, cards[i]);
          inserted = true;
          break;
        }
      }
    }
    if (!inserted) {
      cardGrid.appendChild(card);
    }
  }

  function findVibeById(id) {
    for (var i = 0; i < allVibes.length; i++) {
      if (allVibes[i].id === id) return allVibes[i];
    }
    return null;
  }

  // --- WebSocket vote_update handling ---
  function handleVoteUpdate(payload) {
    // Update in-memory vibe
    for (var i = 0; i < allVibes.length; i++) {
      if (allVibes[i].id === payload.id) {
        allVibes[i].likes = payload.likes;
        allVibes[i].dislikes = payload.dislikes;
        allVibes[i].net_score = payload.net_score;
        break;
      }
    }

    // Update the card's displayed score
    var card = cardGrid.querySelector('[data-vibe-id="' + payload.id + '"]');
    if (card) {
      updateNetScoreDisplay(card, payload.net_score);
    }

    // If sorting by likes, re-sort and re-render
    if (sortMode === 'likes') {
      resortAndRender();
    }
  }

  // --- API Calls ---
  function fetchVibes() {
    fetch('/api/vibes')
      .then(function (res) {
        if (!res.ok) throw new Error('Failed to fetch vibes');
        return res.json();
      })
      .then(function (vibes) {
        allVibes = vibes || [];
        sortVibes(allVibes, sortMode);
        renderVibes(allVibes);
      })
      .catch(function (err) {
        console.error('Error fetching vibes:', err);
        allVibes = [];
        renderVibes([]);
      });
  }

  // --- Connection Status ---
  function setConnected(connected) {
    if (connected) {
      connectionStatus.classList.add('connected');
      connectionStatus.classList.remove('disconnected');
      connectionStatus.querySelector('.status-text').textContent = 'Connected';
    } else {
      connectionStatus.classList.add('disconnected');
      connectionStatus.classList.remove('connected');
      connectionStatus.querySelector('.status-text').textContent = 'Disconnected';
    }
  }

  function updateViewerCount(count) {
    viewerCount.querySelector('.viewer-text').textContent = count + ' listening';
  }

  // --- WebSocket ---
  function connectWebSocket() {
    var protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    var wsUrl = protocol + '//' + window.location.host + '/ws';

    ws = new WebSocket(wsUrl);

    ws.onopen = function () {
      setConnected(true);
      reconnectDelay = 1000;
    };

    ws.onmessage = function (event) {
      try {
        var msg = JSON.parse(event.data);
        if (msg.type === 'new_vibe') {
          // Add to allVibes array
          allVibes.unshift(msg.payload);
          if (sortMode === 'date') {
            prependVibe(msg.payload);
          } else {
            // Sort mode is 'likes' — insert at correct position
            sortVibes(allVibes, sortMode);
            insertVibeAtSortedPosition(msg.payload);
          }
        } else if (msg.type === 'vote_update') {
          handleVoteUpdate(msg.payload);
        } else if (msg.type === 'connected_count') {
          updateViewerCount(msg.payload);
        }
      } catch (err) {
        console.error('Error parsing WebSocket message:', err);
      }
    };

    ws.onclose = function () {
      setConnected(false);
      scheduleReconnect();
    };

    ws.onerror = function () {
      ws.close();
    };
  }

  function scheduleReconnect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
    }
    reconnectTimer = setTimeout(function () {
      reconnectTimer = null;
      connectWebSocket();
    }, reconnectDelay);
    reconnectDelay = Math.min(reconnectDelay * 2, MAX_RECONNECT_DELAY);
  }

  // --- Sort Toggle ---
  sortDateBtn.addEventListener('click', function () {
    if (sortMode === 'date') return;
    sortMode = 'date';
    sortDateBtn.classList.add('active');
    sortLikesBtn.classList.remove('active');
    sortShuffleBtn.classList.remove('active');
    resortAndRender();
  });

  sortLikesBtn.addEventListener('click', function () {
    if (sortMode === 'likes') return;
    sortMode = 'likes';
    sortLikesBtn.classList.add('active');
    sortDateBtn.classList.remove('active');
    sortShuffleBtn.classList.remove('active');
    resortAndRender();
  });

  sortShuffleBtn.addEventListener('click', function () {
    // Fisher-Yates shuffle
    for (var i = allVibes.length - 1; i > 0; i--) {
      var j = Math.floor(Math.random() * (i + 1));
      var temp = allVibes[i];
      allVibes[i] = allVibes[j];
      allVibes[j] = temp;
    }
    sortMode = 'shuffle';
    sortDateBtn.classList.remove('active');
    sortLikesBtn.classList.remove('active');
    sortShuffleBtn.classList.add('active');
    renderVibes(allVibes);
  });

  // --- Form Submission ---
  function clearFormErrors() {
    var existingErrors = postForm.querySelectorAll('.form-error');
    existingErrors.forEach(function (el) { el.remove(); });
  }

  function showFormError(message) {
    clearFormErrors();
    var errorEl = document.createElement('div');
    errorEl.className = 'form-error';
    errorEl.textContent = message;
    errorEl.setAttribute('role', 'alert');
    submitBtn.insertAdjacentElement('beforebegin', errorEl);
  }

  postForm.addEventListener('submit', function (e) {
    e.preventDefault();
    clearFormErrors();

    var url = youtubeUrlInput.value.trim();
    var thought = thoughtInput.value.trim();

    if (!url || !thought) {
      showFormError('Both fields are required.');
      return;
    }

    submitBtn.disabled = true;
    submitBtn.textContent = 'Posting...';

    fetch('/api/vibes', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ youtube_url: url, thought: thought })
    })
      .then(function (res) {
        if (!res.ok) {
          return res.json().then(function (data) {
            throw new Error(data.error || 'Something went wrong');
          });
        }
        return res.json();
      })
      .then(function () {
        youtubeUrlInput.value = '';
        thoughtInput.value = '';
        charCount.textContent = '0 / 150';
        charCount.classList.remove('over-limit');
      })
      .catch(function (err) {
        showFormError(err.message);
      })
      .finally(function () {
        submitBtn.disabled = false;
        submitBtn.textContent = 'Share your earworm 🎶';
      });
  });

  // --- Character Counter ---
  thoughtInput.addEventListener('input', function () {
    var len = thoughtInput.value.length;
    charCount.textContent = len + ' / 150';
    if (len > 150) {
      charCount.classList.add('over-limit');
    } else {
      charCount.classList.remove('over-limit');
    }
  });

  // --- Initialize ---
  if (!cardGrid.classList.contains('vibe-grid')) {
    cardGrid.classList.add('vibe-grid');
  }
  if (!postForm.classList.contains('post-form')) {
    postForm.classList.add('post-form');
  }
  setConnected(false);
  fetchVibes();
  connectWebSocket();

  // --- Expose for testing ---
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = {
      sortVibes: sortVibes,
      getVoteTracker: getVoteTracker,
      setVoteTracker: setVoteTracker,
      hasVoted: hasVoted,
      recordVote: recordVote
    };
  }
})();
