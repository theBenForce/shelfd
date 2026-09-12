import 'dart:async';
import 'dart:js_interop';
import 'package:web/web.dart' as web;
import '../../state/upload_provider.dart';

bool isDirectoryPath(String path) => false;

Future<List<PickedEpubFile>> scanPathForEpubs(String path) async => [];

Future<List<PickedEpubFile>> pickFolderForEpubs() {
  final completer = Completer<List<PickedEpubFile>>();
  final input = web.document.createElement('input') as web.HTMLInputElement;
  input.type = 'file';
  input.setAttribute('webkitdirectory', '');
  input.setAttribute('directory', '');
  input.multiple = true;
  input.style.display = 'none';

  web.document.body?.appendChild(input);

  void cleanup() {
    input.remove();
  }

  input.addEventListener(
    'change',
    (web.Event e) {
      final fileList = input.files;
      if (fileList == null || fileList.length == 0) {
        cleanup();
        if (!completer.isCompleted) completer.complete([]);
        return;
      }

      final epubs = <PickedEpubFile>[];
      for (var i = 0; i < fileList.length; i++) {
        final file = fileList.item(i);
        if (file == null) continue;
        final name = file.name;
        if (name.toLowerCase().endsWith('.epub')) {
          epubs.add(PickedEpubFile(
            name: name,
            readBytes: () async {
              final arrayBuffer = await file.arrayBuffer().toDart;
              return arrayBuffer.toDart.asUint8List();
            },
          ));
        }
      }
      cleanup();
      if (!completer.isCompleted) completer.complete(epubs);
    }.toJS,
  );

  input.addEventListener(
    'cancel',
    (web.Event e) {
      cleanup();
      if (!completer.isCompleted) completer.complete([]);
    }.toJS,
  );

  input.click();

  return completer.future;
}
