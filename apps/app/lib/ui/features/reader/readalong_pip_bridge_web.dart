import 'dart:js_interop';
import 'dart:js_interop_unsafe';
import 'package:web/web.dart' as web;
import 'readalong_pip_models.dart';

ReadalongPipBridge createReadalongPipBridge() => WebReadalongPipBridge();

class WebReadalongPipBridge implements ReadalongPipBridge {
  web.Window? _pipWindow;
  ReadalongPipSnapshot? _currentSnapshot;
  ReadalongPipCallbacks? _callbacks;

  web.HTMLImageElement? _coverEl;
  web.HTMLElement? _titleEl;
  web.HTMLElement? _subtitleEl;
  web.HTMLElement? _stageEl;
  web.HTMLIFrameElement? _iframeEl;
  web.HTMLElement? _reflowEl;
  web.HTMLElement? _progressFillEl;
  web.HTMLElement? _elapsedEl;
  web.HTMLElement? _durationEl;
  web.HTMLButtonElement? _playPauseBtn;
  web.HTMLButtonElement? _speedBtn;

  String? _lastPageUrl;
  String? _lastSegmentsKey;
  int _lastActiveSegmentIndex = -1;
  bool _disposed = false;

  JSObject? get _documentPipApi {
    try {
      if (!web.window.has('documentPictureInPicture')) return null;
      final api = web.window.getProperty<JSAny?>('documentPictureInPicture'.toJS);
      if (api == null || api.isUndefinedOrNull) return null;
      final obj = api as JSObject;
      if (!obj.has('requestWindow')) return null;
      return obj;
    } catch (_) {
      return null;
    }
  }

  @override
  bool get isSupported => _documentPipApi != null;

  @override
  bool get isOpen => _pipWindow != null && !(_pipWindow!.closed);

  @override
  Future<bool> open(
    ReadalongPipSnapshot initial,
    ReadalongPipCallbacks callbacks,
  ) async {
    if (_disposed) return false;
    final pipApi = _documentPipApi;
    if (pipApi == null) return false;

    // requestWindow must be invoked synchronously before any await to keep user activation.
    final JSPromise<JSObject> requestPromise;
    try {
      requestPromise = pipApi.callMethod<JSPromise<JSObject>>(
        'requestWindow'.toJS,
        <String, Object>{'width': 460, 'height': 640}.jsify(),
      );
    } catch (_) {
      return false;
    }

    final web.Window pipWin;
    try {
      pipWin = (await requestPromise.toDart) as web.Window;
    } catch (_) {
      return false;
    }

    if (_disposed) {
      try {
        pipWin.close();
      } catch (_) {}
      return false;
    }

    _detachWindowRefs();
    _pipWindow = pipWin;
    _callbacks = callbacks;
    _currentSnapshot = initial;

    _buildDom(pipWin, initial);
    update(initial);
    return true;
  }

  void _buildDom(web.Window pipWin, ReadalongPipSnapshot initial) {
    final doc = pipWin.document;

    final head = doc.head;
    if (head != null) {
      head.textContent = '';
      final style = doc.createElement('style') as web.HTMLStyleElement;
      style.textContent = _pipStyles;
      head.appendChild(style);
    }
    doc.title = initial.bookTitle.isNotEmpty ? initial.bookTitle : 'Shelfd Read-Along';

    final body = doc.body;
    if (body == null) return;
    body.textContent = '';

    final root = doc.createElement('div') as web.HTMLElement;
    root.className = 'pip-root';

    final header = doc.createElement('header') as web.HTMLElement;
    header.className = 'pip-header';

    final cover = doc.createElement('img') as web.HTMLImageElement;
    cover.className = 'pip-cover';
    cover.alt = '';
    _coverEl = cover;

    final meta = doc.createElement('div') as web.HTMLElement;
    meta.className = 'pip-meta';

    final title = doc.createElement('div') as web.HTMLElement;
    title.className = 'pip-title';
    _titleEl = title;

    final subtitle = doc.createElement('div') as web.HTMLElement;
    subtitle.className = 'pip-subtitle';
    _subtitleEl = subtitle;

    meta.appendChild(title);
    meta.appendChild(subtitle);
    header.appendChild(cover);
    header.appendChild(meta);

    final stage = doc.createElement('main') as web.HTMLElement;
    stage.className = 'pip-stage';
    _stageEl = stage;

    if (initial.isFixedLayout) {
      _mountFixedStage(doc, stage);
    } else {
      _mountReflowStage(doc, stage);
    }

    final footer = doc.createElement('footer') as web.HTMLElement;
    footer.className = 'pip-footer';

    final progressTrack = doc.createElement('div') as web.HTMLElement;
    progressTrack.className = 'pip-progress-track';
    final progressFill = doc.createElement('div') as web.HTMLElement;
    progressFill.className = 'pip-progress-fill';
    progressTrack.appendChild(progressFill);
    _progressFillEl = progressFill;

    progressTrack.addEventListener(
      'click',
      (web.Event event) {
        final snap = _currentSnapshot;
        if (snap == null || snap.duration <= Duration.zero) return;
        final mouseEvent = event as web.MouseEvent;
        final rect = progressTrack.getBoundingClientRect();
        if (rect.width <= 0) return;
        final frac = ((mouseEvent.clientX - rect.left) / rect.width).clamp(0.0, 1.0);
        final targetMs = (snap.duration.inMilliseconds * frac).round();
        _callbacks?.onSeek(Duration(milliseconds: targetMs));
      }.toJS,
    );

    final timesRow = doc.createElement('div') as web.HTMLElement;
    timesRow.className = 'pip-times';
    final elapsed = doc.createElement('span') as web.HTMLElement;
    elapsed.className = 'pip-time';
    final duration = doc.createElement('span') as web.HTMLElement;
    duration.className = 'pip-time';
    _elapsedEl = elapsed;
    _durationEl = duration;
    timesRow.appendChild(elapsed);
    timesRow.appendChild(duration);

    final controls = doc.createElement('div') as web.HTMLElement;
    controls.className = 'pip-controls';

    final skipBackBtn = doc.createElement('button') as web.HTMLButtonElement;
    skipBackBtn.className = 'pip-btn';
    skipBackBtn.type = 'button';
    skipBackBtn.textContent = '-15s';
    skipBackBtn.addEventListener(
      'click',
      (web.Event _) {
        _callbacks?.onSkipBackward();
      }.toJS,
    );

    final playPauseBtn = doc.createElement('button') as web.HTMLButtonElement;
    playPauseBtn.className = 'pip-btn pip-btn-primary';
    playPauseBtn.type = 'button';
    playPauseBtn.addEventListener(
      'click',
      (web.Event _) {
        _callbacks?.onTogglePlayPause();
      }.toJS,
    );
    _playPauseBtn = playPauseBtn;

    final skipFwdBtn = doc.createElement('button') as web.HTMLButtonElement;
    skipFwdBtn.className = 'pip-btn';
    skipFwdBtn.type = 'button';
    skipFwdBtn.textContent = '+30s';
    skipFwdBtn.addEventListener(
      'click',
      (web.Event _) {
        _callbacks?.onSkipForward();
      }.toJS,
    );

    final speedBtn = doc.createElement('button') as web.HTMLButtonElement;
    speedBtn.className = 'pip-btn pip-btn-speed';
    speedBtn.type = 'button';
    speedBtn.addEventListener(
      'click',
      (web.Event _) {
        _callbacks?.onCycleSpeed();
      }.toJS,
    );
    _speedBtn = speedBtn;

    controls.appendChild(skipBackBtn);
    controls.appendChild(playPauseBtn);
    controls.appendChild(skipFwdBtn);
    controls.appendChild(speedBtn);

    footer.appendChild(progressTrack);
    footer.appendChild(timesRow);
    footer.appendChild(controls);

    root.appendChild(header);
    root.appendChild(stage);
    root.appendChild(footer);
    body.appendChild(root);

    pipWin.addEventListener(
      'message',
      (web.Event event) {
        _handleWindowMessage(event as web.MessageEvent);
      }.toJS,
    );

    pipWin.addEventListener(
      'pagehide',
      (web.Event _) {
        final onClosed = _callbacks?.onClosed;
        _detachWindowRefs();
        onClosed?.call();
      }.toJS,
    );
  }

  void _mountFixedStage(web.Document doc, web.HTMLElement stage) {
    stage.textContent = '';
    _reflowEl = null;
    final iframe = doc.createElement('iframe') as web.HTMLIFrameElement;
    iframe.id = 'shelfd-pip-frame';
    iframe.className = 'pip-iframe';
    iframe.setAttribute('frameBorder', '0');
    iframe.setAttribute('scrolling', 'no');
    iframe.addEventListener(
      'load',
      (web.Event _) {
        final snap = _currentSnapshot;
        if (snap != null) {
          _postHighlight(snap);
        }
      }.toJS,
    );
    stage.appendChild(iframe);
    _iframeEl = iframe;
  }

  void _mountReflowStage(web.Document doc, web.HTMLElement stage) {
    stage.textContent = '';
    _iframeEl = null;
    final container = doc.createElement('div') as web.HTMLElement;
    container.className = 'pip-reflow';
    stage.appendChild(container);
    _reflowEl = container;
  }

  void _handleWindowMessage(web.MessageEvent event) {
    final data = event.data;
    if (data == null || data.isUndefinedOrNull) return;
    try {
      final obj = data as JSObject;
      if (!obj.has('type')) return;
      final type = obj.getProperty<JSAny?>('type'.toJS);
      if (type == null || !type.isA<JSString>()) return;
      if ((type as JSString).toDart != 'shelfd:readalong-click') return;

      final fragVal = obj.getProperty<JSAny?>('fragmentId'.toJS);
      if (fragVal == null || !fragVal.isA<JSString>()) return;
      final fragmentId = (fragVal as JSString).toDart.trim();
      if (fragmentId.isEmpty) return;

      final snap = _currentSnapshot;
      if (snap == null) return;
      for (final seg in snap.segments) {
        if (seg.fragmentId == fragmentId) {
          _callbacks?.onSeek(
            Duration(milliseconds: (seg.globalBegin * 1000).round()),
          );
          return;
        }
      }
    } catch (_) {}
  }

  void _postHighlight(ReadalongPipSnapshot snapshot) {
    final iframe = _iframeEl;
    if (iframe == null) return;
    final payload = <String, Object?>{
      'type': 'shelfd:readalong-highlight',
      'bookId': snapshot.bookId,
      'chapterHref': snapshot.chapterHref,
      'fragmentId': snapshot.fragmentId,
      'activeClass': snapshot.activeClass,
      'playbackActiveClass': snapshot.playbackActiveClass,
      'playing': snapshot.isPlaying,
    }.jsify();
    try {
      iframe.contentWindow?.postMessage(payload, '*'.toJS);
    } catch (_) {}
  }

  @override
  void update(ReadalongPipSnapshot snapshot) {
    if (!isOpen) return;
    _currentSnapshot = snapshot;

    final pipWin = _pipWindow!;
    final doc = pipWin.document;

    final nextTitle = snapshot.bookTitle.isNotEmpty ? snapshot.bookTitle : 'Shelfd Read-Along';
    if (doc.title != nextTitle) {
      doc.title = nextTitle;
    }
    _titleEl?.textContent = snapshot.bookTitle;
    final subtitleParts = <String>[
      if (snapshot.chapterTitle.isNotEmpty) snapshot.chapterTitle,
      if (snapshot.authorDisplay.isNotEmpty && snapshot.chapterTitle.isEmpty)
        snapshot.authorDisplay,
    ];
    _subtitleEl?.textContent = subtitleParts.join(' · ');

    final coverEl = _coverEl;
    if (coverEl != null) {
      final coverUrl = snapshot.coverUrl;
      if (coverUrl != null && coverUrl.isNotEmpty) {
        if (coverEl.src != coverUrl) {
          coverEl.src = coverUrl;
        }
        coverEl.style.display = 'block';
      } else {
        coverEl.style.display = 'none';
      }
    }

    final stage = _stageEl;
    if (stage != null) {
      if (snapshot.isFixedLayout) {
        if (_iframeEl == null) {
          _lastPageUrl = null;
          _mountFixedStage(doc, stage);
        }
        final pageUrl = snapshot.pageUrl;
        if (pageUrl != null && pageUrl.isNotEmpty && pageUrl != _lastPageUrl) {
          _lastPageUrl = pageUrl;
          _iframeEl!.src = pageUrl;
        }
        _postHighlight(snapshot);
      } else {
        if (_reflowEl == null) {
          _lastSegmentsKey = null;
          _lastActiveSegmentIndex = -1;
          _mountReflowStage(doc, stage);
        }
        _updateReflowSegments(doc, snapshot);
      }
    }

    final frac = snapshot.duration.inMilliseconds > 0
        ? (snapshot.position.inMilliseconds / snapshot.duration.inMilliseconds)
            .clamp(0.0, 1.0)
        : 0.0;
    _progressFillEl?.style.width = '${(frac * 100).toStringAsFixed(2)}%';
    _elapsedEl?.textContent = _formatDuration(snapshot.position);
    _durationEl?.textContent = _formatDuration(snapshot.duration);

    final playPauseBtn = _playPauseBtn;
    if (playPauseBtn != null) {
      playPauseBtn.textContent = snapshot.isPlaying ? 'Pause' : 'Play';
      playPauseBtn.setAttribute(
        'aria-label',
        snapshot.isPlaying ? 'Pause playback' : 'Play playback',
      );
    }

    _speedBtn?.textContent = _formatSpeed(snapshot.speed);
  }

  void _updateReflowSegments(web.Document doc, ReadalongPipSnapshot snapshot) {
    final container = _reflowEl;
    if (container == null) return;

    final segmentsKey = _computeSegmentsKey(snapshot.segments);
    if (segmentsKey != _lastSegmentsKey) {
      _lastSegmentsKey = segmentsKey;
      _lastActiveSegmentIndex = -1;
      container.textContent = '';

      if (snapshot.segments.isEmpty) {
        final empty = doc.createElement('p') as web.HTMLElement;
        empty.className = 'pip-empty';
        empty.textContent = 'No synced text available for this section.';
        container.appendChild(empty);
      } else {
        for (var i = 0; i < snapshot.segments.length; i++) {
          final seg = snapshot.segments[i];
          final p = doc.createElement('p') as web.HTMLElement;
          p.className = 'seg';
          p.setAttribute('data-idx', '$i');
          p.textContent = seg.text;
          final beginMs = (seg.globalBegin * 1000).round();
          p.addEventListener(
            'click',
            (web.Event _) {
              _callbacks?.onSeek(Duration(milliseconds: beginMs));
            }.toJS,
          );
          container.appendChild(p);
        }
      }
    }

    final activeIdx = snapshot.activeSegmentIndex;
    if (activeIdx != _lastActiveSegmentIndex) {
      if (_lastActiveSegmentIndex >= 0) {
        final prevEl = container.querySelector('p.seg[data-idx="$_lastActiveSegmentIndex"]');
        prevEl?.classList.remove('active');
      }
      _lastActiveSegmentIndex = activeIdx;
      if (activeIdx >= 0) {
        final nextEl = container.querySelector('p.seg[data-idx="$activeIdx"]');
        if (nextEl != null) {
          nextEl.classList.add('active');
          try {
            (nextEl as JSObject).callMethod<JSAny?>(
              'scrollIntoView'.toJS,
              <String, Object>{'behavior': 'smooth', 'block': 'center'}.jsify(),
            );
          } catch (_) {}
        }
      }
    }
  }

  String _computeSegmentsKey(List<ReadalongPipSegment> segments) {
    if (segments.isEmpty) return 'empty';
    return '${segments.length}:${segments.first.chapterIndex}:${segments.first.fragmentId}:${segments.first.globalBegin}:${segments.last.globalBegin}';
  }

  String _formatDuration(Duration d) {
    final totalSeconds = d.inSeconds < 0 ? 0 : d.inSeconds;
    final hours = totalSeconds ~/ 3600;
    final minutes = (totalSeconds % 3600) ~/ 60;
    final seconds = totalSeconds % 60;
    final mm = minutes.toString().padLeft(2, '0');
    final ss = seconds.toString().padLeft(2, '0');
    if (hours > 0) {
      return '$hours:$mm:$ss';
    }
    return '$mm:$ss';
  }

  String _formatSpeed(double speed) {
    final fixed = speed.toStringAsFixed(2);
    if (fixed.endsWith('0')) {
      return '${speed.toStringAsFixed(1)}x';
    }
    return '${fixed}x';
  }

  void _detachWindowRefs() {
    _pipWindow = null;
    _coverEl = null;
    _titleEl = null;
    _subtitleEl = null;
    _stageEl = null;
    _iframeEl = null;
    _reflowEl = null;
    _progressFillEl = null;
    _elapsedEl = null;
    _durationEl = null;
    _playPauseBtn = null;
    _speedBtn = null;
    _lastPageUrl = null;
    _lastSegmentsKey = null;
    _lastActiveSegmentIndex = -1;
  }

  @override
  void close() {
    final win = _pipWindow;
    _detachWindowRefs();
    if (win != null && !win.closed) {
      try {
        win.close();
      } catch (_) {}
    }
  }

  @override
  void dispose() {
    if (_disposed) return;
    _disposed = true;
    _callbacks = null;
    _currentSnapshot = null;
    close();
  }
}

const String _pipStyles = '''
* { box-sizing: border-box; margin: 0; padding: 0; }
html, body {
  width: 100%;
  height: 100%;
  background: #161513;
  color: #F4EFE6;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  overflow: hidden;
}
.pip-root {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
}
.pip-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  background: #22201D;
  border-bottom: 1px solid rgba(244, 239, 230, 0.1);
  flex-shrink: 0;
}
.pip-cover {
  width: 32px;
  height: 44px;
  border-radius: 4px;
  object-fit: cover;
  flex-shrink: 0;
  background: #2E2B27;
}
.pip-meta {
  min-width: 0;
  flex: 1;
}
.pip-title {
  font-size: 13px;
  font-weight: 600;
  color: #F4EFE6;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.pip-subtitle {
  font-size: 11px;
  color: #A8A095;
  margin-top: 2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.pip-stage {
  flex: 1;
  min-height: 0;
  position: relative;
  background: #161513;
}
.pip-iframe {
  width: 100%;
  height: 100%;
  border: 0;
  display: block;
  background: #FFFFFF;
}
.pip-reflow {
  width: 100%;
  height: 100%;
  overflow-y: auto;
  padding: 16px 18px;
  line-height: 1.65;
  font-family: Georgia, "Times New Roman", serif;
  font-size: 16px;
}
.pip-reflow .seg {
  padding: 6px 8px;
  margin-bottom: 6px;
  border-radius: 6px;
  cursor: pointer;
  color: #DDD5C7;
  transition: background-color 140ms ease, color 140ms ease;
}
.pip-reflow .seg:hover {
  background: rgba(244, 239, 230, 0.06);
}
.pip-reflow .seg.active {
  background: rgba(217, 119, 87, 0.24);
  color: #FFFFFF;
}
.pip-empty {
  color: #A8A095;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  font-size: 13px;
  text-align: center;
  margin-top: 32px;
}
.pip-footer {
  padding: 10px 14px 12px;
  background: #22201D;
  border-top: 1px solid rgba(244, 239, 230, 0.1);
  flex-shrink: 0;
}
.pip-progress-track {
  width: 100%;
  height: 6px;
  background: rgba(244, 239, 230, 0.14);
  border-radius: 999px;
  cursor: pointer;
  overflow: hidden;
}
.pip-progress-fill {
  height: 100%;
  width: 0%;
  background: #D97757;
  border-radius: 999px;
}
.pip-times {
  display: flex;
  justify-content: space-between;
  margin-top: 5px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  color: #A8A095;
}
.pip-controls {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin-top: 8px;
}
.pip-btn {
  appearance: none;
  border: 1px solid rgba(244, 239, 230, 0.16);
  background: #2C2925;
  color: #F4EFE6;
  border-radius: 999px;
  padding: 6px 12px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
}
.pip-btn:hover {
  background: #38342F;
}
.pip-btn-primary {
  background: #D97757;
  border-color: #D97757;
  color: #161513;
  font-weight: 600;
  min-width: 72px;
}
.pip-btn-primary:hover {
  background: #E2876A;
}
.pip-btn-speed {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  min-width: 54px;
}
''';
