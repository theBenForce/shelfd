import 'dart:io';
import 'dart:typed_data';
import 'package:file_picker/file_picker.dart';

List<String> _inferAllowedExtensions(String fileName, String? mimeType) {
  final lower = fileName.toLowerCase();
  if (lower.endsWith('.md') ||
      lower.endsWith('.markdown') ||
      (mimeType != null && mimeType.contains('markdown'))) {
    return ['md', 'markdown', 'txt'];
  }
  if (lower.endsWith('.txt') ||
      (mimeType != null && mimeType.startsWith('text/plain'))) {
    return ['txt'];
  }
  return ['epub'];
}

String _inferDialogTitle(String fileName, String? mimeType) {
  final lower = fileName.toLowerCase();
  if (lower.endsWith('.md') ||
      lower.endsWith('.markdown') ||
      (mimeType != null && mimeType.contains('markdown'))) {
    return 'Save NotebookLM Markdown';
  }
  return 'Save EPUB';
}

Future<bool> saveDownloadedFile(
  List<int> bytes,
  String fileName, {
  String? mimeType,
}) async {
  if (Platform.environment.containsKey('FLUTTER_TEST')) {
    return true;
  }
  final uint8List = bytes is Uint8List ? bytes : Uint8List.fromList(bytes);
  final allowedExtensions = _inferAllowedExtensions(fileName, mimeType);
  final dialogTitle = _inferDialogTitle(fileName, mimeType);

  Uri? uri;
  try {
    uri = await FilePicker.saveFile(
      dialogTitle: dialogTitle,
      fileName: fileName,
      type: FileType.custom,
      allowedExtensions: allowedExtensions,
      bytes: uint8List,
    );
  } catch (_) {
    return false;
  }
  if (uri == null) {
    return false;
  }
  try {
    final filePath = uri.toFilePath();
    if (filePath.isNotEmpty) {
      final file = File(filePath);
      if (!await file.exists() || await file.length() != uint8List.length) {
        await file.writeAsBytes(uint8List);
      }
    }
  } catch (_) {
    // If toFilePath fails or already saved, ignore
  }
  return true;
}

Future<bool> openExternalUrl(String url) async {
  final trimmed = url.trim();
  if (trimmed.isEmpty) return false;
  if (Platform.environment.containsKey('FLUTTER_TEST')) {
    return true;
  }
  try {
    if (Platform.isMacOS) {
      await Process.run('open', [trimmed]);
      return true;
    } else if (Platform.isLinux) {
      await Process.run('xdg-open', [trimmed]);
      return true;
    } else if (Platform.isWindows) {
      await Process.run('cmd', ['/c', 'start', '', trimmed]);
      return true;
    }
  } catch (_) {
    // Fall through on headless or unsupported IO platforms
  }
  return false;
}
