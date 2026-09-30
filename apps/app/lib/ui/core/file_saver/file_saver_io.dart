import 'dart:io';
import 'dart:typed_data';
import 'package:file_picker/file_picker.dart';

Future<bool> saveDownloadedFile(
  List<int> bytes,
  String fileName, {
  String mimeType = 'application/epub+zip',
}) async {
  final uint8List = bytes is Uint8List ? bytes : Uint8List.fromList(bytes);
  final uri = await FilePicker.saveFile(
    dialogTitle: 'Save EPUB',
    fileName: fileName,
    type: FileType.custom,
    allowedExtensions: ['epub'],
    bytes: uint8List,
  );
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
