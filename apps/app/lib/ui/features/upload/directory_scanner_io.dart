import 'dart:io';
import 'package:file_picker/file_picker.dart';
import '../../state/upload_provider.dart';

bool isDirectoryPath(String path) {
  try {
    return FileSystemEntity.typeSync(path) == FileSystemEntityType.directory;
  } catch (_) {
    return false;
  }
}

Future<List<PickedEpubFile>> scanPathForEpubs(String path) async {
  final results = <PickedEpubFile>[];
  try {
    final type = FileSystemEntity.typeSync(path);
    if (type == FileSystemEntityType.directory) {
      final dir = Directory(path);
      final entities = dir.listSync(recursive: true, followLinks: false);
      for (final entity in entities) {
        if (entity is File && entity.path.toLowerCase().endsWith('.epub')) {
          final filename = entity.uri.pathSegments.isNotEmpty
              ? entity.uri.pathSegments.last
              : entity.path.split(Platform.pathSeparator).last;
          results.add(PickedEpubFile(
            name: filename,
            path: entity.path,
            readBytes: () => entity.readAsBytes(),
          ));
        }
      }
    } else if (type == FileSystemEntityType.file && path.toLowerCase().endsWith('.epub')) {
      final file = File(path);
      final filename = file.uri.pathSegments.isNotEmpty
          ? file.uri.pathSegments.last
          : path.split(Platform.pathSeparator).last;
      results.add(PickedEpubFile(
        name: filename,
        path: path,
        readBytes: () => file.readAsBytes(),
      ));
    }
  } catch (_) {}
  return results;
}

Future<List<PickedEpubFile>> pickFolderForEpubs() async {
  final dirPath = await FilePicker.getDirectoryPath();
  if (dirPath == null || dirPath.trim().isEmpty) {
    return [];
  }
  return scanPathForEpubs(dirPath);
}
