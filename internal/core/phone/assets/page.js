// The phone page (SPEC §10). The server owns the state; this script draws it.
//
// The page is in one of these views:
//   waiting  no submission is waiting for photos
//   ready    a submission is waiting: exercises, photos, Send (sending is
//            ready with the buttons off)
//   sent     the photos were sent; stays until the next submission is offered
//   expired  the link ended; nothing works any more
//
// The server sends {v, state: "waiting"|"ready", title, offer, exercises}. The
// page asks GET <base>/state?v=<last v>, which the server holds until
// something changes (long polling, about 25 s), and asks again at once. A
// response older than the newest one seen (v lower) is ignored. When the page
// becomes visible again, or the network failed, it asks again (with a backoff
// after a failure).
(function () {
  'use strict';

  var app = document.getElementById('app');
  var text = JSON.parse(document.getElementById('text').textContent);
  var maxPhotos = parseInt(app.getAttribute('data-max'), 10) || 6;
  var base = location.pathname.replace(/\/+$/, '');

  var titleEl = document.getElementById('title');
  var offlineEl = document.getElementById('offline');
  var waitingPanel = document.getElementById('waiting');
  var noticeEl = document.getElementById('notice');
  var capture = document.getElementById('capture');
  var exercisesEl = document.getElementById('exercises');
  var sentPanel = document.getElementById('sent');
  var expiredPanel = document.getElementById('expired');
  var input = document.getElementById('file');
  var takeLabel = document.getElementById('take');
  var thumbs = document.getElementById('thumbs');
  var count = document.getElementById('count');
  var message = document.getElementById('message');
  var progress = document.getElementById('progress');
  var sendButton = document.getElementById('send');

  var view = ''; // waiting, ready, sent or expired
  var version = 0; // the newest v seen
  var shownOffer = 0; // the offer the ready view shows
  var photos = []; // { file, url }
  var sending = false;

  function fill(template, values) {
    return template.replace(/\{(\w+)\}/g, function (all, name) {
      return Object.prototype.hasOwnProperty.call(values, name) ? String(values[name]) : all;
    });
  }

  function show(name) {
    view = name;
    waitingPanel.hidden = name !== 'waiting';
    capture.hidden = name !== 'ready';
    sentPanel.hidden = name !== 'sent';
    expiredPanel.hidden = name !== 'expired';
    titleEl.hidden = name === 'expired' || !titleEl.textContent;
    if (name === 'expired') {
      offlineEl.hidden = true;
    }
  }

  function setTitle(title) {
    titleEl.textContent = title || '';
    titleEl.hidden = !title || view === 'expired';
    document.title = title ? title + ' - Radio Tutor' : 'Radio Tutor';
  }

  function showMessage(msg) {
    message.textContent = msg || '';
    message.hidden = !msg;
  }

  function showNotice(msg) {
    noticeEl.textContent = msg || '';
    noticeEl.hidden = !msg;
  }

  function dropPhotos() {
    photos.forEach(function (photo) {
      URL.revokeObjectURL(photo.url);
    });
    photos = [];
    sending = false;
    progress.hidden = true;
    showMessage('');
  }

  // ---- the ready view ----------------------------------------------------

  function renderExercises(list) {
    while (exercisesEl.firstChild) {
      exercisesEl.removeChild(exercisesEl.firstChild);
    }
    (list || []).forEach(function (ex) {
      var item = document.createElement('li');
      item.className = 'exercise';
      var badge = document.createElement('span');
      badge.className = 'badge';
      badge.textContent = ex.label;
      var body = document.createElement('div');
      body.className = 'exercise-body';
      var h3 = document.createElement('h3');
      h3.textContent = ex.title;
      body.appendChild(h3);
      if (ex.lines && ex.lines.length > 0) {
        var p = document.createElement('p');
        p.className = 'prompt';
        ex.lines.forEach(function (line, i) {
          if (i > 0) {
            p.appendChild(document.createElement('br'));
          }
          p.appendChild(document.createTextNode(line));
        });
        body.appendChild(p);
      }
      item.appendChild(badge);
      item.appendChild(body);
      exercisesEl.appendChild(item);
    });
  }

  function renderPhotos() {
    while (thumbs.firstChild) {
      thumbs.removeChild(thumbs.firstChild);
    }
    photos.forEach(function (photo, index) {
      var item = document.createElement('li');
      item.className = 'thumb';
      var img = document.createElement('img');
      img.src = photo.url;
      img.alt = '';
      var remove = document.createElement('button');
      remove.type = 'button';
      remove.textContent = '✕';
      remove.setAttribute('aria-label', text['phone.remove']);
      remove.disabled = sending;
      remove.addEventListener('click', function () {
        removePhoto(index);
      });
      item.appendChild(img);
      item.appendChild(remove);
      thumbs.appendChild(item);
    });
    count.hidden = photos.length === 0;
    count.textContent = fill(text['phone.count'], { n: photos.length, max: maxPhotos });
    takeLabel.textContent = photos.length === 0 ? text['phone.take'] : text['phone.take_more'];
    takeLabel.hidden = photos.length >= maxPhotos;
    input.disabled = sending;
    sendButton.disabled = sending || photos.length === 0;
    sendButton.textContent = sending ? text['phone.sending'] : text['phone.send'];
  }

  function removePhoto(index) {
    if (sending) {
      return;
    }
    var removed = photos.splice(index, 1);
    if (removed.length > 0) {
      URL.revokeObjectURL(removed[0].url);
    }
    showMessage('');
    renderPhotos();
  }

  input.addEventListener('change', function () {
    var files = Array.prototype.slice.call(input.files || []);
    var tooMany = false;
    files.forEach(function (file) {
      if (photos.length >= maxPhotos) {
        tooMany = true;
        return;
      }
      photos.push({ file: file, url: URL.createObjectURL(file) });
    });
    // Reset the input so that the same photo can be taken again.
    input.value = '';
    showMessage(tooMany ? fill(text['phone.error.too_many'], { max: maxPhotos }) : '');
    renderPhotos();
  });

  function enterReady(st) {
    dropPhotos();
    shownOffer = st.offer;
    renderExercises(st.exercises);
    renderPhotos();
    showNotice('');
    show('ready');
  }

  function enterWaiting(notice) {
    dropPhotos();
    shownOffer = 0;
    showNotice(notice);
    show('waiting');
  }

  function enterSent() {
    dropPhotos();
    shownOffer = 0;
    show('sent');
  }

  function enterExpired() {
    dropPhotos();
    stopped = true;
    show('expired');
  }

  // apply draws a state of the server, unless a newer one was drawn already.
  function apply(st) {
    if (st.expired) {
      enterExpired();
      return;
    }
    if (st.v < version) {
      return;
    }
    version = st.v;
    setTitle(st.title);
    if (st.state === 'ready') {
      if (view === 'ready' && shownOffer === st.offer) {
        return; // the same submission: keep the photos taken so far
      }
      if (sending) {
        return; // an upload decides what comes next
      }
      enterReady(st);
      return;
    }
    // waiting
    if (view === 'ready' && !sending) {
      enterWaiting('');
    } else if (view === '') {
      enterWaiting('');
    }
    // sent stays until the next offer; a sending page waits for its upload.
  }

  // ---- sending -----------------------------------------------------------

  sendButton.addEventListener('click', function () {
    if (sending || photos.length === 0) {
      return;
    }
    sending = true;
    showMessage('');
    progress.value = 0;
    progress.hidden = false;
    renderPhotos();

    var form = new FormData();
    photos.forEach(function (photo, index) {
      form.append('photos', photo.file, photo.file.name || 'photo-' + (index + 1) + '.jpg');
    });

    var xhr = new XMLHttpRequest();
    xhr.open('POST', base + '/photos?offer=' + encodeURIComponent(shownOffer));
    xhr.upload.onprogress = function (event) {
      if (event.lengthComputable && event.total > 0) {
        progress.value = Math.round((event.loaded / event.total) * 100);
      }
    };
    xhr.onload = function () {
      var reply = {};
      try {
        reply = JSON.parse(xhr.responseText);
      } catch (e) {
        reply = {};
      }
      if (xhr.status >= 200 && xhr.status < 300 && reply.ok) {
        enterSent();
        poll();
      } else if (reply.expired || xhr.status === 410) {
        enterExpired();
      } else if (reply.stale) {
        enterWaiting(reply.error || text['phone.error.stale']);
        poll();
      } else {
        sending = false;
        progress.hidden = true;
        showMessage(reply.error || text['phone.error.server']);
        renderPhotos();
      }
    };
    xhr.onerror = function () {
      sending = false;
      progress.hidden = true;
      showMessage(text['phone.error.network']);
      renderPhotos();
    };
    xhr.ontimeout = xhr.onerror;
    xhr.send(form);
  });

  // ---- long polling ------------------------------------------------------

  var stopped = false;
  var pollXhr = null;
  var pollTimer = null;
  var backoff = 0;

  function fail(delay) {
    offlineEl.hidden = false;
    backoff = delay || Math.min(backoff ? backoff * 2 : 1000, 15000);
    pollTimer = setTimeout(poll, backoff);
  }

  function poll() {
    if (stopped) {
      return;
    }
    clearTimeout(pollTimer);
    if (pollXhr) {
      var old = pollXhr;
      pollXhr = null;
      old.abort();
    }
    var started = Date.now();
    var asked = version;
    var xhr = new XMLHttpRequest();
    pollXhr = xhr;
    xhr.open('GET', base + '/state?v=' + version);
    xhr.timeout = 45000; // the server holds a request for about 25 s
    xhr.onload = function () {
      if (xhr !== pollXhr) {
        return; // replaced by a newer request
      }
      pollXhr = null;
      var st = null;
      try {
        st = JSON.parse(xhr.responseText);
      } catch (e) {
        st = null;
      }
      if (xhr.status === 410 || (st && st.expired)) {
        enterExpired();
        return;
      }
      if (xhr.status === 429) {
        fail((parseInt(xhr.getResponseHeader('Retry-After'), 10) || 5) * 1000);
        return;
      }
      if (xhr.status !== 200 || !st) {
        fail();
        return;
      }
      backoff = 0;
      offlineEl.hidden = true;
      apply(st);
      // An answer that came at once with nothing new must not make a tight loop.
      var quick = Date.now() - started < 1000 && st.v === asked;
      pollTimer = setTimeout(poll, quick ? 1000 : 0);
    };
    xhr.onerror = function () {
      if (xhr === pollXhr) {
        pollXhr = null;
        fail();
      }
    };
    xhr.ontimeout = xhr.onerror;
    xhr.send();
  }

  document.addEventListener('visibilitychange', function () {
    if (!document.hidden && !stopped) {
      backoff = 0;
      poll();
    }
  });

  // ---- start -------------------------------------------------------------

  apply(JSON.parse(document.getElementById('state').textContent));
  poll();
})();
