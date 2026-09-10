import 'dart:ui_web' as ui_web;
import 'package:flutter/material.dart';
import 'package:web/web.dart' as web;

final Set<String> _registeredViews = <String>{};
final Map<String, web.HTMLIFrameElement> _activeIframes = <String, web.HTMLIFrameElement>{};

Widget buildHtmlFrame({
  required String viewKey,
  required String url,
  required double width,
  required double height,
}) {
  final uniqueKey = '${viewKey}_${url.hashCode}';
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
        _activeIframes[uniqueKey] = iframe;
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
