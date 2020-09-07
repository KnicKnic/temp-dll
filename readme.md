# GOAL

Allow a golang library to be dependent on windows dlls and hide this from library consumers.

The reason I want to do this a lot of people prefer single file distributables. This code allows that. They embed the dll in the binary, and this will have some helper functions to load it. It accomplishes this by writing the binary to a temporary file. It remains for consideration in the future to use https://github.com/fancycode/MemoryModule. 

~~Currently the code follows the pattern from https://www.drdobbs.com/packing-dlls-in-your-exe/184416443 (write file, open file with delete on close, load library).~~ Cannot due this due to loadlibrary not allowing FILE_SHARE_DELETE

Currently this code copies the dll into windows temp directory. It does so with the following pattern.

```text
    tempdirectory/(base32 ecoded sha256)-(originalfilename)
```

The copied dll stays there forever.


