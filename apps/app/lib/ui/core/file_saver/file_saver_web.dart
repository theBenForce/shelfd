import 'dart:js_interop';
import 'dart:typed_data';
import 'package:web/web.dart' as web;

String _inferMimeType(String fileName, String? mimeType) {
  if (mimeType != null && mimeType.isNotEmpty) {
    return mimeType;
  }
  final lower = fileName.toLowerCase();
  if (lower.endsWith('.md') || lower.endsWith('.markdown')) {
    return 'text/markdown;charset=utf-8';
  }
  if (lower.endsWith('.txt')) {
    return 'text/plain;charset=utf-8';
  }
  return 'application/epub+zip';
}

Future<bool> saveDownloadedFile(
  List<int> bytes,
  String fileName, {
  String? mimeType,
}) async {
  final effectiveMimeType = _inferMimeType(fileName, mimeType);
  final uint8List = bytes is Uint8List ? bytes : Uint8List.fromList(bytes);
  final blob = web.Blob(
    [uint8List.toJS].toJS,
    web.BlobPropertyBag(type: effectiveMimeType),
  );
  final url = web.URL.createObjectURL(blob);
  final anchor = web.document.createElement('a') as web.HTMLAnchorElement;
  anchor.href = url;
  anchor.download = fileName;
  anchor.style.display = 'none';
  web.document.body?.appendChild(anchor);
  anchor.click();
  anchor.remove();
  web.URL.revokeObjectURL(url);
  return true;
}

Future<bool> openExternalUrl(String url) async {
  if (url.trim().isEmpty) return false;
  try {
    web.window.open(url, '_blank');
    return true;
  } catch (_) {
    return false;
  }
}
