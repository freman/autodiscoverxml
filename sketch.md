golang tool to handle autodiscover.xml ("/autodiscover/autodiscover.xml") requests... with sanity and fallbacks. and templates!

handle all cases and forms of autodiscover (case insensitive, both in /autodiscover/ and not)

based on this old php script but we're not running php any more

<?php
function giveup() {
        header($_SERVER['SERVER_PROTOCOL'] . ' 400 Bad Request');
        echo "400 - Bad request\n";
        exit;
}

if ($_SERVER['REQUEST_METHOD'] != 'POST')
        giveup();

$raw = file_get_contents('php://input');
if (strlen($raw) == 0)
        giveup();

$xml = @simplexml_load_string($raw);
if ($xml === false)
        giveup();

$email = @$xml->Request->EMailAddress;
if (strpos($email, '@') === false)
        giveup();

header('Content-Type: application/xml');
?>
<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/responseschema/2006">
  <Response xmlns="http://schemas.microsoft.com/exchange/autodiscover/outlook/responseschema/2006a">
    <User>
      <DisplayName>Fremnet</DisplayName>
    </User>
    <Account>
      <AccountType>email</AccountType>
      <Action>settings</Action>
      <Protocol>
        <Type>IMAP</Type>
        <Server>mail.fremnet.net</Server>
        <Port>993</Port>
        <DomainRequired>off</DomainRequired>
        <SPA>off</SPA>
        <SSL>on</SSL>
        <AuthRequired>on</AuthRequired>
        <LoginName><?php echo $email; ?></LoginName>
      </Protocol>
      <Protocol>
        <Type>SMTP</Type>
        <Server>mail.fremnet.net</Server>
        <Port>465</Port>
        <DomainRequired>off</DomainRequired>
        <SPA>off</SPA>
        <SSL>on</SSL>
        <AuthRequired>on</AuthRequired>
        <LoginName><?php echo $email; ?></LoginName>
      </Protocol>
    </Account>
  </Response>
</Autodiscover>


Obviously turn that into more of a template... provide an example template, make mine non-comitable..

on the initial post be aware there might be soap wrapped requests...
eg: <?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"
               xmlns:a="http://schemas.microsoft.com/exchange/autodiscover/outlook/requestschema/2006">
  <soap:Body>
    <a:Autodiscover>
      <a:Request>
        <a:EMailAddress>user@example.com</a:EMailAddress>
        <a:AcceptableResponseSchema>
          http://schemas.microsoft.com/exchange/autodiscover/outlook/responseschema/2006a
        </a:AcceptableResponseSchema>
      </a:Request>
    </a:Autodiscover>
  </soap:Body>
</soap:Envelope>

but may also be like <?xml version="1.0" encoding="utf-8"?>
<Autodiscover>
  <Request>
    <EMailAddress>user@example.com</EMailAddress>
  </Request>
</Autodiscover>

or <Request>
  <EmailAddress>user@example.com</EmailAddress>
</Request>


as well as the more correct <?xml version="1.0" encoding="utf-8" ?>
<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/outlook/requestschema/2006">
  <Request>
    <AcceptableResponseSchema>http://schemas.microsoft.com/exchange/autodiscover/outlook/responseschema/2006a</AcceptableResponseSchema>
    <EMailAddress>JohnDoe@sample.com</EMailAddress>
  </Request>
</Autodiscover>

make it capable of picking the autodiscover template from the host header, safely... 

de-case the incomming tags, make them all lowercase

make the email address into an object that is stringable so we can template it as {{email}} {{email.user}} {{email.domain}}

not stressed about client error handling if they can't handle a 400 without crashing that's not on me.

header verification could just check to see if the file exists have it be under a path that I can provide as a command line argument, there's no reason for anything that wouldnt be in a domain to be even qieried so skip \ and / at least if not everything else... if the file doesn't exist return whatever t he most appropriate 400 is, probably 418

if we can't figure out what the request is really about (soap or otherwise) without causing an anurism, then 400 it all the way

absolutly add handling for mozilla's auto config aswell, same delio, templates, with examples - handle providing email via argument for mozilla... because obviously different protocol doing same thing

mayube {strict path argument}/$domain/autoconfig.xml and {strict path argument}/$domain/config-v1.1.xml

re displayname concerns that's the point of templating... if I choose I could make displayname Fremnet, {{email}}, {{email.dommain}} - {{email.user}}

I don't think we need toml/yaml just use go templates, maybe use the html one so it's less likely to choke on xml... or write one that handles an xml template (can validate it) but that sounds like a lot of hard work given we might want optionals or something lol so stick with go html template

strip the host header down to it's domain, it'll probably be autoconfig.fremnet.net or something so strip it to that.

deliberately ignore acceptableresponseschema if we don't know it I'm not getting paid to do this and neither are you lol

