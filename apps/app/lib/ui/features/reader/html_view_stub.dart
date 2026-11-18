import 'package:flutter/material.dart';

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

Widget buildHtmlFrame({
  required String viewKey,
  required String url,
  required double width,
  required double height,
}) {
  return Container(
    key: Key('html-frame-$viewKey'),
    width: width,
    height: height,
    color: Colors.black,
    child: const Center(
      child: Text(
        'Fixed-layout view is optimized for web or embedded browser.',
        style: TextStyle(color: Colors.white70),
      ),
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
  _lastHighlight = ReadalongFrameHighlight(
    targetViewKey: targetViewKey,
    bookId: bookId,
    chapterHref: chapterHref,
    fragmentId: fragmentId,
    activeClass: activeClass,
    playbackActiveClass: playbackActiveClass,
    playing: playing,
  );
}

