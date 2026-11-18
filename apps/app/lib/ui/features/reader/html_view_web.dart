import 'dart:js_interop';
import 'dart:ui_web' as ui_web;
import 'package:flutter/material.dart';
import 'package:web/web.dart' as web;

class ReadalongFrameHighlight {
  final String? targetViewKey;
  final String? bookId;
  final String? chapterHref;
  final String fragmentId;
  final String activeClass;
  final String playbackActiveClass;
  final bool playing;

  const ReadalongFrameHighlight({
    this.targetViewKey,
    this.bookId,
    this.chapterHref,
    required this.fragmentId,
    this.activeClass = '-epub-media-overlay-active',
    this.playbackActiveClass = '-epub-media-overlay-playing',
    this.playing = true,
  });
}

final Set<String> _registeredViews = <String>{};
final Map<String, web.HTMLIFrameElement> _activeIframes = <String, web.HTMLIFrameElement>{};
final Map<String, String> _iframeViewKeys = <String, String>{};
ReadalongFrameHighlight? _lastHighlight;

ReadalongFrameHighlight? get lastReadalongHighlight => _lastHighlight;

String activeFragmentForViewKey(String viewKey) {
  final hl = _lastHighlight;
  if (hl == null || hl.fragmentId.isEmpty) return '';
  if (hl.targetViewKey != null && hl.targetViewKey != viewKey) return '';
  return hl.fragmentId;
}

void resetHtmlFrameHighlightsForTest() {
  _lastHighlight = null;
}

void _postHighlightToIframe(
  String iframeViewKey,
  web.HTMLIFrameElement iframe,
  ReadalongFrameHighlight hl,
) {
  final isTarget = hl.fragmentId.isNotEmpty &&
      (hl.targetViewKey == null || hl.targetViewKey == iframeViewKey);
  final payload = <String, Object?>{
    'type': 'shelfd:readalong-highlight',
    'bookId': hl.bookId ?? '',
    'chapterHref': isTarget ? (hl.chapterHref ?? '') : '',
    'fragmentId': isTarget ? hl.fragmentId : '',
    'activeClass': hl.activeClass,
    'playbackActiveClass': hl.playbackActiveClass,
    'playing': isTarget ? hl.playing : false,
  }.jsify();

  try {
    iframe.contentWindow?.postMessage(payload, '*'.toJS);
  } catch (_) {}
}

Widget buildHtmlFrame({
  required String viewKey,
  required String url,
  required double width,
  required double height,
}) {
  final uniqueKey = '${viewKey}_${url.hashCode}';
  _iframeViewKeys[uniqueKey] = viewKey;
  if (!_registeredViews.contains(uniqueKey)) {
    _registeredViews.add(uniqueKey);
    ui_web.platformViewRegistry.registerViewFactory(
      uniqueKey,
      (int id) {
        final iframe = web.document.createElement('iframe') as web.HTMLIFrameElement;
        iframe.src = url;
        iframe.style.border = 'none';
        iframe.style.width = '100%';
        iframe.style.height = '100%';
        iframe.style.overflow = 'hidden';
        iframe.setAttribute('scrolling', 'no');
        iframe.setAttribute('frameBorder', '0');
        iframe.addEventListener(
          'load',
          (web.Event _) {
            final hl = _lastHighlight;
            if (hl != null) {
              _postHighlightToIframe(viewKey, iframe, hl);
            }
          }.toJS,
        );
        _activeIframes[uniqueKey] = iframe;
        _iframeViewKeys[uniqueKey] = viewKey;
        return iframe;
      },
    );
  } else {
    final existing = _activeIframes[uniqueKey];
    if (existing != null && existing.src != url) {
      existing.src = url;
    }
  }

  return SizedBox(
    width: width,
    height: height,
    child: HtmlElementView(
      key: ValueKey(uniqueKey),
      viewType: uniqueKey,
    ),
  );
}

void highlightHtmlFrame({
  String? targetViewKey,
  String? bookId,
  String? chapterHref,
  required String fragmentId,
  String activeClass = '-epub-media-overlay-active',
  String playbackActiveClass = '-epub-media-overlay-playing',
  bool playing = true,
}) {
  final hl = ReadalongFrameHighlight(
    targetViewKey: targetViewKey,
    bookId: bookId,
    chapterHref: chapterHref,
    fragmentId: fragmentId,
    activeClass: activeClass,
    playbackActiveClass: playbackActiveClass,
    playing: playing,
  );
  _lastHighlight = hl;

  for (final entry in _activeIframes.entries) {
    final iframeViewKey = _iframeViewKeys[entry.key] ?? '';
    _postHighlightToIframe(iframeViewKey, entry.value, hl);
  }
}
