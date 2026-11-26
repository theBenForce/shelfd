import 'dart:collection';
import 'dart:convert';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:webview_flutter/webview_flutter.dart';
import 'package:webview_flutter_android/webview_flutter_android.dart';
import '../../../platform/qr_scanner_service.dart';
import 'html_view_models.dart';
import 'html_view_stub.dart' as stub;

export 'html_view_models.dart';

const int _maxLiveWebViewControllers = 2;
final LinkedHashMap<String, _ManagedWebViewEntry> _liveControllers =
    LinkedHashMap<String, _ManagedWebViewEntry>();

ReadalongFrameHighlight? _lastHighlight;

bool shouldUseHtmlViewStubForCurrentEnvironment() {
  if (Platform.environment.containsKey('FLUTTER_TEST')) {
    return true;
  }
  if (!Platform.isAndroid && !Platform.isIOS) {
    return true;
  }
  if (WebViewPlatform.instance == null) {
    return true;
  }
  return false;
}

bool get requiresNativeMediaCookie =>
    !shouldUseHtmlViewStubForCurrentEnvironment();

ReadalongFrameHighlight? get lastReadalongHighlight =>
    shouldUseHtmlViewStubForCurrentEnvironment()
        ? stub.lastReadalongHighlight
        : _lastHighlight;

String activeFragmentForViewKey(String viewKey) {
  if (shouldUseHtmlViewStubForCurrentEnvironment()) {
    return stub.activeFragmentForViewKey(viewKey);
  }
  final hl = _lastHighlight;
  if (hl == null || hl.fragmentId.isEmpty) return '';
  if (hl.targetViewKey != null && hl.targetViewKey != viewKey) return '';
  return hl.fragmentId;
}

void resetHtmlFrameHighlightsForTest() {
  _lastHighlight = null;
  stub.resetHtmlFrameHighlightsForTest();
}

Future<void> seedHtmlAuthCookie({
  required String serverBaseUrl,
  required String mediaJwt,
}) async {
  if (shouldUseHtmlViewStubForCurrentEnvironment()) {
    return;
  }
  final trimmedBase = serverBaseUrl.trim();
  final trimmedJwt = mediaJwt.trim();
  if (trimmedBase.isEmpty || trimmedJwt.isEmpty) {
    return;
  }
  final uri = Uri.tryParse(trimmedBase);
  if (uri == null || !uri.hasScheme || uri.host.isEmpty) {
    return;
  }

  final isSecure = uri.scheme.toLowerCase() == 'https';
  final origin = '${uri.scheme}://${uri.authority}';
  final cookieHeader =
      'shelfd_token=${Uri.encodeComponent(trimmedJwt)}; Path=/; HttpOnly; SameSite=Lax${isSecure ? '; Secure' : ''}';

  if (Platform.isAndroid) {
    final applied = await QrScannerService().setWebViewCookie(
      url: origin,
      cookie: cookieHeader,
    );
    if (!applied) {
      throw StateError('Failed to apply HttpOnly media cookie on Android WebView.');
    }
    return;
  }

  await WebViewCookieManager().setCookie(
    WebViewCookie(
      name: 'shelfd_token',
      value: trimmedJwt,
      domain: uri.host,
      path: '/',
    ),
  );
}

Future<void> clearHtmlAuthCookies() async {
  if (Platform.environment.containsKey('FLUTTER_TEST')) {
    return;
  }
  if (Platform.isAndroid) {
    await QrScannerService().clearWebViewCookies();
  }
  if ((Platform.isAndroid || Platform.isIOS) && WebViewPlatform.instance != null) {
    try {
      await WebViewCookieManager().clearCookies();
    } catch (_) {}
  }
}

Widget buildHtmlFrame({
  required String viewKey,
  required String url,
  required double width,
  required double height,
  OnReadalongClick? onReadalongClick,
  OnToggleControls? onToggleControls,
  OnPageDimensions? onPageDimensions,
}) {
  if (shouldUseHtmlViewStubForCurrentEnvironment()) {
    return stub.buildHtmlFrame(
      viewKey: viewKey,
      url: url,
      width: width,
      height: height,
      onReadalongClick: onReadalongClick,
      onToggleControls: onToggleControls,
      onPageDimensions: onPageDimensions,
    );
  }

  return SizedBox(
    key: Key('html-frame-$viewKey'),
    width: width,
    height: height,
    child: _NativeEpubWebViewFrame(
      key: ValueKey<String>(viewKey),
      viewKey: viewKey,
      url: url,
      width: width,
      height: height,
      onReadalongClick: onReadalongClick,
      onToggleControls: onToggleControls,
      onPageDimensions: onPageDimensions,
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

  if (shouldUseHtmlViewStubForCurrentEnvironment()) {
    stub.highlightHtmlFrame(
      targetViewKey: targetViewKey,
      bookId: bookId,
      chapterHref: chapterHref,
      fragmentId: fragmentId,
      activeClass: activeClass,
      playbackActiveClass: playbackActiveClass,
      playing: playing,
    );
    return;
  }

  for (final entry in _liveControllers.values) {
    entry.applyHighlight(hl);
  }
}

class _ManagedWebViewEntry {
  final String viewKey;
  final WebViewController controller;
  String loadedUrl;
  double targetWidth;
  double targetHeight;
  OnReadalongClick? onReadalongClick;
  OnToggleControls? onToggleControls;
  OnPageDimensions? onPageDimensions;

  _ManagedWebViewEntry({
    required this.viewKey,
    required this.controller,
    required this.loadedUrl,
    required this.targetWidth,
    required this.targetHeight,
    this.onReadalongClick,
    this.onToggleControls,
    this.onPageDimensions,
  });

  void applyViewportFit() {
    if (targetWidth <= 0 || targetHeight <= 0) return;
    final tw = targetWidth;
    final th = targetHeight;
    // Normalizes legacy fixed-pixel viewport metas from older servers and supplies
    // the exact Flutter logical box when WebView layout dimensions settle asynchronously.
    final script = '''
(function(tw, th) {
  if (!document || !document.documentElement) return;
  window.__shelfdTargetWidth = tw;
  window.__shelfdTargetHeight = th;
  if (typeof window.__shelfdFitPage === 'function') {
    window.__shelfdFitPage(tw, th);
    return;
  }
  var root = document.documentElement;
  var savedW = parseFloat(root.getAttribute('data-shelfd-dw') || '0');
  var savedH = parseFloat(root.getAttribute('data-shelfd-dh') || '0');
  var metas = document.querySelectorAll('meta[name="viewport"]');
  for (var i = 0; i < metas.length; i++) {
    var content = metas[i].getAttribute('content') || '';
    if (savedW <= 0 || savedH <= 0) {
      var wm = content.match(/width\\s*=\\s*(\\d+)/i);
      var hm = content.match(/height\\s*=\\s*(\\d+)/i);
      if (wm && hm) {
        savedW = parseFloat(wm[1]);
        savedH = parseFloat(hm[1]);
      }
    }
    metas[i].setAttribute('content', 'width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no');
  }
  if (!document.body) return;
  if (savedW <= 0 || savedH <= 0) {
    var bs = document.body.getAttribute('style') || '';
    var bwm = bs.match(/(?:^|[;\\s])width\\s*:\\s*([0-9.]+)\\s*px/i);
    var bhm = bs.match(/(?:^|[;\\s])height\\s*:\\s*([0-9.]+)\\s*px/i);
    if (bwm && bhm) {
      savedW = parseFloat(bwm[1]);
      savedH = parseFloat(bhm[1]);
    }
  }
  if (savedW <= 0 || savedH <= 0) {
    var svg = document.querySelector('svg[viewBox]');
    if (svg) {
      var vb = (svg.getAttribute('viewBox') || '').trim().split(/[\\s,]+/);
      if (vb.length === 4) {
        savedW = parseFloat(vb[2]);
        savedH = parseFloat(vb[3]);
      }
    }
  }
  if (savedW <= 0 || savedH <= 0) {
    var sw = document.body.scrollWidth || 0;
    var sh = document.body.scrollHeight || 0;
    if (sw > tw + 8 || sh > th + 8) {
      savedW = Math.max(sw, tw);
      savedH = Math.max(sh, th);
    }
  }
  if (savedW > 0 && savedH > 0) {
    root.setAttribute('data-shelfd-dw', String(savedW));
    root.setAttribute('data-shelfd-dh', String(savedH));
    var scale = Math.min(tw / savedW, th / savedH);
    var ox = Math.max(0, (tw - savedW * scale) / 2);
    var oy = Math.max(0, (th - savedH * scale) / 2);
    root.style.setProperty('width', '100%', 'important');
    root.style.setProperty('height', '100%', 'important');
    root.style.setProperty('overflow', 'hidden', 'important');
    var style = document.body.style;
    style.setProperty('width', savedW + 'px', 'important');
    style.setProperty('height', savedH + 'px', 'important');
    style.setProperty('position', 'absolute', 'important');
    style.setProperty('left', ox + 'px', 'important');
    style.setProperty('top', oy + 'px', 'important');
    style.setProperty('transform-origin', '0 0', 'important');
    style.setProperty('transform', 'scale(' + scale + ')', 'important');
    style.setProperty('overflow', 'hidden', 'important');
    if (window.ShelfdBridge && typeof window.ShelfdBridge.postMessage === 'function') {
      try {
        window.ShelfdBridge.postMessage(JSON.stringify({
          type: 'shelfd:page-dimensions',
          width: Math.round(savedW),
          height: Math.round(savedH)
        }));
      } catch (_) {}
    }
  }
})(${tw.toStringAsFixed(2)}, ${th.toStringAsFixed(2)});
''';
    controller.runJavaScript(script).catchError((_) {});
  }

  void applyHighlight(ReadalongFrameHighlight hl) {
    final isTarget = hl.fragmentId.isNotEmpty &&
        (hl.targetViewKey == null || hl.targetViewKey == viewKey);
    final effectiveFragment = isTarget ? hl.fragmentId : '';
    final effectivePlaying = isTarget ? hl.playing : false;

    final script =
        'if (window.__shelfdHighlightReadalong) { '
        'window.__shelfdHighlightReadalong('
        '${jsonEncode(effectiveFragment)}, '
        '${jsonEncode(hl.activeClass)}, '
        '${jsonEncode(hl.playbackActiveClass)}, '
        '${effectivePlaying ? 'true' : 'false'}'
        '); }';
    controller.runJavaScript(script).catchError((_) {});
  }
}

class _NativeEpubWebViewFrame extends StatefulWidget {
  final String viewKey;
  final String url;
  final double width;
  final double height;
  final OnReadalongClick? onReadalongClick;
  final OnToggleControls? onToggleControls;
  final OnPageDimensions? onPageDimensions;

  const _NativeEpubWebViewFrame({
    super.key,
    required this.viewKey,
    required this.url,
    required this.width,
    required this.height,
    this.onReadalongClick,
    this.onToggleControls,
    this.onPageDimensions,
  });

  @override
  State<_NativeEpubWebViewFrame> createState() => _NativeEpubWebViewFrameState();
}

class _NativeEpubWebViewFrameState extends State<_NativeEpubWebViewFrame> {
  late _ManagedWebViewEntry _entry;

  @override
  void initState() {
    super.initState();
    _entry = _acquireControllerEntry();
  }

  @override
  void didUpdateWidget(covariant _NativeEpubWebViewFrame oldWidget) {
    super.didUpdateWidget(oldWidget);
    _entry.onReadalongClick = widget.onReadalongClick;
    _entry.onToggleControls = widget.onToggleControls;
    _entry.onPageDimensions = widget.onPageDimensions;
    final sizeChanged = (oldWidget.width - widget.width).abs() > 0.5 ||
        (oldWidget.height - widget.height).abs() > 0.5;
    _entry.targetWidth = widget.width;
    _entry.targetHeight = widget.height;
    if (_entry.loadedUrl != widget.url && widget.url.isNotEmpty) {
      _entry.loadedUrl = widget.url;
      final parsed = Uri.tryParse(widget.url);
      if (parsed != null && parsed.hasScheme) {
        _entry.controller.loadRequest(parsed);
      }
    } else if (sizeChanged) {
      _entry.applyViewportFit();
    }
  }

  @override
  void dispose() {
    _liveControllers.remove(widget.viewKey);
    super.dispose();
  }

  _ManagedWebViewEntry _acquireControllerEntry() {
    final existing = _liveControllers.remove(widget.viewKey);
    if (existing != null) {
      existing.onReadalongClick = widget.onReadalongClick;
      existing.onToggleControls = widget.onToggleControls;
      existing.onPageDimensions = widget.onPageDimensions;
      final sizeChanged = (existing.targetWidth - widget.width).abs() > 0.5 ||
          (existing.targetHeight - widget.height).abs() > 0.5;
      existing.targetWidth = widget.width;
      existing.targetHeight = widget.height;
      _liveControllers[widget.viewKey] = existing;
      if (existing.loadedUrl != widget.url) {
        existing.loadedUrl = widget.url;
        final parsed = Uri.tryParse(widget.url);
        if (parsed != null && parsed.hasScheme) {
          existing.controller.loadRequest(parsed);
        }
      } else if (sizeChanged) {
        existing.applyViewportFit();
      }
      return existing;
    }

    while (_liveControllers.length >= _maxLiveWebViewControllers) {
      _liveControllers.remove(_liveControllers.keys.first);
    }

    final controller = WebViewController();
    controller.setJavaScriptMode(JavaScriptMode.unrestricted);
    controller.setBackgroundColor(Colors.white);

    final platformController = controller.platform;
    if (platformController is AndroidWebViewController) {
      platformController.setUseWideViewPort(false);
      platformController.enableZoom(false);
      platformController.setTextZoom(100);
      platformController.setVerticalScrollBarEnabled(false);
      platformController.setHorizontalScrollBarEnabled(false);
      platformController.setAllowFileAccess(false);
      platformController.setAllowContentAccess(false);
    }

    late final _ManagedWebViewEntry created;
    controller.addJavaScriptChannel(
      'ShelfdBridge',
      onMessageReceived: (JavaScriptMessage msg) {
        try {
          final decoded = jsonDecode(msg.message);
          if (decoded is! Map<String, dynamic>) return;
          final type = decoded['type']?.toString() ?? '';
          if (type == 'shelfd:readalong-click') {
            final fragmentId = decoded['fragmentId']?.toString() ?? '';
            final cb = created.onReadalongClick;
            if (cb != null && fragmentId.isNotEmpty) {
              cb(fragmentId);
            } else {
              created.onToggleControls?.call();
            }
          } else if (type == 'shelfd:toggle-controls') {
            created.onToggleControls?.call();
          } else if (type == 'shelfd:page-dimensions') {
            final w = (decoded['width'] as num?)?.toDouble() ?? 0;
            final h = (decoded['height'] as num?)?.toDouble() ?? 0;
            if (w > 0 && h > 0) {
              created.onPageDimensions?.call(w, h);
            }
          }
        } catch (_) {}
      },
    );

    controller.setNavigationDelegate(
      NavigationDelegate(
        onPageFinished: (_) {
          created.applyViewportFit();
          Future<void>.delayed(const Duration(milliseconds: 120), () {
            if (_liveControllers[created.viewKey] == created) {
              created.applyViewportFit();
            }
          });
          final hl = _lastHighlight;
          if (hl != null) {
            created.applyHighlight(hl);
          }
        },
        onNavigationRequest: (NavigationRequest request) {
          if (request.url == created.loadedUrl || request.url == 'about:blank') {
            return NavigationDecision.navigate;
          }
          return NavigationDecision.prevent;
        },
      ),
    );

    created = _ManagedWebViewEntry(
      viewKey: widget.viewKey,
      controller: controller,
      loadedUrl: widget.url,
      targetWidth: widget.width,
      targetHeight: widget.height,
      onReadalongClick: widget.onReadalongClick,
      onToggleControls: widget.onToggleControls,
      onPageDimensions: widget.onPageDimensions,
    );
    _liveControllers[widget.viewKey] = created;

    final parsed = Uri.tryParse(widget.url);
    if (parsed != null && parsed.hasScheme) {
      controller.loadRequest(parsed);
    }

    return created;
  }

  @override
  Widget build(BuildContext context) {
    return WebViewWidget(controller: _entry.controller);
  }
}
