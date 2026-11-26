import 'package:flutter/foundation.dart';

typedef OnReadalongClick = void Function(String fragmentId);
typedef OnToggleControls = VoidCallback;
typedef OnPageDimensions = void Function(double width, double height);

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
