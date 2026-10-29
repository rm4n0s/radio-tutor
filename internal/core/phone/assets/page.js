(function () {
  'use strict';

  var app = document.getElementById('app');
  var text = JSON.parse(document.getElementById('text').textContent);
  var maxPhotos = parseInt(app.getAttribute('data-max'), 10) || 6;
  var input = document.getElementById('file');
  var takeLabel = document.getElementById('take');
  var thumbs = document.getElementById('thumbs');
  var count = document.getElementById('count');
  var message = document.getElementById('message');
  var progress = document.getElementById('progress');
  var sendButton = document.getElementById('send');
  var capture = document.getElementById('capture');
  var sentPanel = document.getElementById('sent');
  var expiredPanel = document.getElementById('expired');

  var photos = []; // { file, url }
  var sending = false;

  function fill(template, values) {
    return template.replace(/\{(\w+)\}/g, function (all, name) {
      return Object.prototype.hasOwnProperty.call(values, name) ? String(values[name]) : all;
    });
  }

  function showMessage(msg) {
    if (msg) {
      message.textContent = msg;
      message.hidden = false;
    } else {
      message.textContent = '';
      message.hidden = true;
    }
  }

  function render() {
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
    render();
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
    render();
  });

  function showExpired() {
    capture.hidden = true;
    sentPanel.hidden = true;
    expiredPanel.hidden = false;
  }

  function showSent() {
    photos.forEach(function (photo) {
      URL.revokeObjectURL(photo.url);
    });
    photos = [];
    capture.hidden = true;
    expiredPanel.hidden = true;
    sentPanel.hidden = false;
  }

  function failed(msg) {
    sending = false;
    progress.hidden = true;
    showMessage(msg);
    render();
  }

  sendButton.addEventListener('click', function () {
    if (sending || photos.length === 0) {
      return;
    }
    sending = true;
    showMessage('');
    progress.value = 0;
    progress.hidden = false;
    render();

    var form = new FormData();
    photos.forEach(function (photo, index) {
      form.append('photos', photo.file, photo.file.name || 'photo-' + (index + 1) + '.jpg');
    });

    var xhr = new XMLHttpRequest();
    xhr.open('POST', location.pathname.replace(/\/+$/, '') + '/photos');
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
        showSent();
      } else if (reply.expired || xhr.status === 410) {
        showExpired();
      } else {
        failed(reply.error || text['phone.error.server']);
      }
    };
    xhr.onerror = function () {
      failed(text['phone.error.network']);
    };
    xhr.ontimeout = xhr.onerror;
    xhr.send(form);
  });

  render();
})();
